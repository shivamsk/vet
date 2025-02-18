package filter

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"

	"github.com/golang/protobuf/jsonpb"
	"github.com/google/cel-go/cel"
	"github.com/safedep/dry/utils"
	"github.com/safedep/vet/gen/filterinput"
	"github.com/safedep/vet/gen/insightapi"
	"github.com/safedep/vet/pkg/common/logger"
	"github.com/safedep/vet/pkg/models"
)

const (
	filterInputVarRoot      = "_"
	filterInputVarPkg       = "pkg"
	filterInputVarVulns     = "vulns"
	filterInputVarScorecard = "scorecard"
	filterInputVarProjects  = "projects"
	filterInputVarLicenses  = "licenses"

	// Soft limit to start with
	filterEvalMaxFilters = 50
)

var (
	errMaxFilter = errors.New("max filter limit has reached")
)

type Evaluator interface {
	AddFilter(name, filter string) error
	EvalPackage(pkg *models.Package) (*filterEvaluationResult, error)
}

type filterEvaluator struct {
	name        string
	env         *cel.Env
	programs    []*filterProgram
	ignoreError bool
}

func NewEvaluator(name string, ignoreError bool) (Evaluator, error) {
	env, err := cel.NewEnv(
		cel.Variable(filterInputVarPkg, cel.DynType),
		cel.Variable(filterInputVarVulns, cel.DynType),
		cel.Variable(filterInputVarProjects, cel.DynType),
		cel.Variable(filterInputVarScorecard, cel.DynType),
		cel.Variable(filterInputVarLicenses, cel.DynType),
		cel.Variable(filterInputVarRoot, cel.DynType),
	)

	if err != nil {
		return nil, err
	}

	return &filterEvaluator{
		name:        name,
		env:         env,
		programs:    []*filterProgram{},
		ignoreError: ignoreError,
	}, nil
}

func (f *filterEvaluator) AddFilter(name, filter string) error {
	if len(f.programs) >= filterEvalMaxFilters {
		return errMaxFilter
	}

	ast, issues := f.env.Compile(filter)
	if issues != nil && issues.Err() != nil {
		return issues.Err()
	}

	prog, err := f.env.Program(ast)
	if err != nil {
		return err
	}

	f.programs = append(f.programs, &filterProgram{
		name:    name,
		program: prog,
	})

	return nil
}

func (f *filterEvaluator) EvalPackage(pkg *models.Package) (*filterEvaluationResult, error) {
	filterInput, err := f.buildFilterInput(pkg)
	if err != nil {
		return nil, err
	}

	serializedInput, err := f.serializeFilterInput(filterInput)
	if err != nil {
		return nil, err
	}

	for _, prog := range f.programs {
		out, _, err := prog.program.Eval(map[string]interface{}{
			filterInputVarRoot:      serializedInput,
			filterInputVarPkg:       serializedInput["pkg"],
			filterInputVarProjects:  serializedInput["projects"],
			filterInputVarVulns:     serializedInput["vulns"],
			filterInputVarScorecard: serializedInput["scorecard"],
			filterInputVarLicenses:  serializedInput["licenses"],
		})

		if err != nil {
			logger.Warnf("CEL evaluator error: %s", err.Error())

			if f.ignoreError {
				continue
			}

			return nil, err
		}

		if (reflect.TypeOf(out).Kind() == reflect.Bool) &&
			(reflect.ValueOf(out).Bool()) {

			return &filterEvaluationResult{
				match:   true,
				program: prog,
			}, nil
		}
	}

	return &filterEvaluationResult{
		match: false,
	}, nil
}

// TODO: Fix this JSON round-trip problem by directly configuring CEL env to
// work with Protobuf messages
func (f *filterEvaluator) serializeFilterInput(fi *filterinput.FilterInput) (map[string]interface{}, error) {
	var ret map[string]interface{}
	m := jsonpb.Marshaler{OrigName: true, EnumsAsInts: false, EmitDefaults: true}

	data, err := m.MarshalToString(fi)
	if err != nil {
		return ret, err
	}

	logger.Debugf("Serialized filter input: %s", data)

	err = json.Unmarshal([]byte(data), &ret)
	if err != nil {
		return ret, err
	}

	return ret, nil
}

func (f *filterEvaluator) buildFilterInput(pkg *models.Package) (*filterinput.FilterInput, error) {
	fi := filterinput.FilterInput{
		Pkg: &filterinput.PackageVersion{
			Ecosystem: strings.ToLower(string(pkg.PackageDetails.Ecosystem)),
			Name:      pkg.PackageDetails.Name,
			Version:   pkg.PackageDetails.Version,
		},
		Projects: []*filterinput.ProjectInfo{},
		Vulns: &filterinput.Vulnerabilities{
			All:      []*filterinput.Vulnerability{},
			Critical: []*filterinput.Vulnerability{},
			High:     []*filterinput.Vulnerability{},
			Medium:   []*filterinput.Vulnerability{},
			Low:      []*filterinput.Vulnerability{},
		},
		Scorecard: &filterinput.Scorecard{
			Scores: map[string]float32{},
		},
		Licenses: []string{},
	}

	// Safely get insight
	insight := utils.SafelyGetValue(pkg.Insights)

	// Add projects
	projectTypeMapper := func(tp string) filterinput.ProjectType {
		tp = strings.ToLower(tp)
		if tp == "github" {
			return filterinput.ProjectType_GITHUB
		} else {
			return filterinput.ProjectType_UNKNOWN
		}
	}

	for _, project := range utils.SafelyGetValue(insight.Projects) {
		fi.Projects = append(fi.Projects, &filterinput.ProjectInfo{
			Name:   utils.SafelyGetValue(project.Name),
			Stars:  int32(utils.SafelyGetValue(project.Stars)),
			Forks:  int32(utils.SafelyGetValue(project.Forks)),
			Issues: int32(utils.SafelyGetValue(project.Issues)),
			Type:   projectTypeMapper(utils.SafelyGetValue(project.Type)),
		})
	}

	// Add vulnerabilities
	cveFilter := func(aliases []string) string {
		for _, alias := range aliases {
			if strings.HasPrefix(strings.ToUpper(alias), "CVE-") {
				return alias
			}
		}

		return ""
	}

	for _, vuln := range utils.SafelyGetValue(insight.Vulnerabilities) {
		fiv := filterinput.Vulnerability{
			Id:  utils.SafelyGetValue(vuln.Id),
			Cve: cveFilter(utils.SafelyGetValue(vuln.Aliases)),
		}

		fi.Vulns.All = append(fi.Vulns.All, &fiv)

		risk := insightapi.PackageVulnerabilitySeveritiesRiskUNKNOWN
		for _, s := range utils.SafelyGetValue(vuln.Severities) {
			sType := utils.SafelyGetValue(s.Type)
			if (sType == insightapi.PackageVulnerabilitySeveritiesTypeCVSSV3) ||
				(sType == insightapi.PackageVulnerabilitySeveritiesTypeCVSSV2) {
				risk = utils.SafelyGetValue(s.Risk)
				break
			}
		}

		switch risk {
		case insightapi.PackageVulnerabilitySeveritiesRiskCRITICAL:
			fi.Vulns.Critical = append(fi.Vulns.Critical, &fiv)
			break
		case insightapi.PackageVulnerabilitySeveritiesRiskHIGH:
			fi.Vulns.High = append(fi.Vulns.High, &fiv)
			break
		case insightapi.PackageVulnerabilitySeveritiesRiskMEDIUM:
			fi.Vulns.Medium = append(fi.Vulns.Medium, &fiv)
			break
		case insightapi.PackageVulnerabilitySeveritiesRiskLOW:
			fi.Vulns.Low = append(fi.Vulns.Low, &fiv)
			break
		}
	}

	// Add licenses
	for _, lic := range utils.SafelyGetValue(insight.Licenses) {
		fi.Licenses = append(fi.Licenses, string(lic))
	}

	// Scorecard
	scorecard := utils.SafelyGetValue(insight.Scorecard)
	scorecardContent := utils.SafelyGetValue(scorecard.Content)

	// Aggregated score
	fi.Scorecard.Score = utils.SafelyGetValue(scorecardContent.Score)

	checks := utils.SafelyGetValue(scorecardContent.Checks)
	for _, check := range checks {
		fi.Scorecard.Scores[string(utils.SafelyGetValue(check.Name))] =
			utils.SafelyGetValue(check.Score)
	}

	return &fi, nil
}
