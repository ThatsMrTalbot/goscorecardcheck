package goscorecardcheck

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/thatsmrtalbot/goscorecardcheck/internal/scorecard"
	"golang.org/x/mod/modfile"
)

const goModFilename = "go.mod"

type Rule struct {
	// Check to check minimum score against, if unset will check against the
	// overall score
	Check string `yaml:"check,omitempty"`
	// The minimum allowed score
	MinimumScore float64 `yaml:"minimumScore"`
}

// Policy a policy to apply when validating scorecards
type Policy struct {
	// Description of the policy
	Description string `yaml:"description"`
	// Packages to include, supports wildcards in paths
	Include []string `yaml:"include,omitempty"`
	// Packages to exclude, supports wildcards in paths
	Exclude []string `yaml:"exclude,omitempty"`
	// Rules for this policy
	Rules []Rule `yaml:"rules"`
}

// Configuration of for allowing/blocking dependencies using its
// https://github.com/ossf/scorecard score
type Configuration struct {
	Policies []Policy `yaml:"policies"`
}

// Processor processes go files
type Processor struct {
	Config  *Configuration
	Modfile *modfile.File

	getter scorecard.Getter
}

// NewProcessor will create a Processor to lint blocked packages.
func NewProcessor(config *Configuration) (*Processor, error) {
	goModFileBytes, err := loadGoModFile()
	if err != nil {
		return nil, fmt.Errorf("unable to read module file %s: %w", goModFilename, err)
	}

	modFile, err := modfile.Parse(goModFilename, goModFileBytes, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to parse module file %s: %w", goModFilename, err)
	}

	p := &Processor{
		Config:  config,
		Modfile: modFile,
	}

	return p, nil
}

// ProcessFiles takes a string slice with file names (full paths)
// and lints them.
func (p *Processor) ProcessFiles(ctx context.Context, filenames []string) (issues []Issue) {
	for _, filename := range filenames {
		data, err := os.ReadFile(filename)
		if err != nil {
			issues = append(issues, Issue{
				FileName:   filename,
				LineNumber: 0,
				Reason:     fmt.Sprintf("unable to read file, file cannot be linted: %s", err.Error()),
			})

			continue
		}

		issues = append(issues, p.process(ctx, filename, data)...)
	}

	return issues
}

// process file imports and add lint error if blocked package is imported.
func (p *Processor) process(ctx context.Context, filename string, data []byte) (issues []Issue) {
	fileSet := token.NewFileSet()

	file, err := parser.ParseFile(fileSet, filename, data, parser.ParseComments)
	if err != nil {
		issues = append(issues, Issue{
			FileName:   filename,
			LineNumber: 0,
			Reason:     fmt.Sprintf("invalid syntax, file cannot be linted (%s)", err.Error()),
		})

		return
	}

	imports := file.Imports
	for n := range imports {
		importedPkg := strings.TrimSpace(strings.Trim(imports[n].Path.Value, "\""))

		blockReasons, err := p.isBlockedPackage(ctx, importedPkg)
		if err != nil {
			issues = append(issues, p.addError(fileSet, imports[n].Pos(), fmt.Sprintf("unable to get scorecard for package: %s", err)))
		}

		if blockReasons == nil {
			continue
		}

		for _, blockReason := range blockReasons {
			issues = append(issues, p.addError(fileSet, imports[n].Pos(), blockReason))
		}
	}

	return issues
}

// addError adds an error for the file and line number for the current token.Pos
// with the given reason.
func (p *Processor) addError(fileset *token.FileSet, pos token.Pos, reason string) Issue {
	position := fileset.Position(pos)

	return Issue{
		FileName:   position.Filename,
		LineNumber: position.Line,
		Position:   position,
		Reason:     reason,
	}
}

func (p *Processor) isBlockedPackage(ctx context.Context, pkg string) (reasons []string, _ error) {
	// Std library
	if !strings.Contains(pkg, ".") {
		return nil, nil
	}

	// Don't check for same module
	if strings.HasPrefix(pkg, p.Modfile.Module.Mod.Path) {
		return nil, nil
	}

	// Get dependency for a given path
	dep, err := scorecard.ParseDependencyForImportPath(pkg)
	if err != nil {
		return nil, err
	}

	// Loop over policies, finding ones that apply
	for _, policy := range p.Config.Policies {
		// By default we match all packages
		keep := true

		// If an "include" list is provided, we default keep to false and match
		// each of the include statements.
		if len(policy.Include) > 0 {
			keep = false
			for _, include := range policy.Include {
				match, err := doublestar.Match(strings.ToLower(include), strings.ToLower(dep.Root))
				if err != nil {
					return nil, fmt.Errorf("invalid package match %q: %w", include, err)
				}

				if match {
					keep = true
					break
				}
			}
		}

		// If the package is marked to keep, check each of the exclude
		// statements to see if it should be excluded
		if keep {
			for _, exclude := range policy.Exclude {
				match, err := doublestar.Match(strings.ToLower(exclude), strings.ToLower(dep.Root))
				if err != nil {
					return nil, fmt.Errorf("invalid package match %q: %w", exclude, err)
				}

				if match {
					keep = false
					break
				}
			}
		}

		// Policy does not apply, continue
		if !keep {
			continue
		}

		// Get the score for a given package, the getter will ensure this only ever
		// results in a single API call
		score, err := p.getter.Get(ctx, dep)
		if err != nil {
			return nil, err
		}

		// Test each policy rule
		for _, rule := range policy.Rules {
			var s float64
			var d string

			if rule.Check != "" {
				d = fmt.Sprintf(" for check %q", rule.Check)
				for _, check := range score.Checks {
					if check.Name == rule.Check {
						s = check.Score
						break
					}
				}
			} else {
				s = score.Score
			}

			if s < rule.MinimumScore {
				reasons = append(reasons, fmt.Sprintf("import of package %q is blocked: %s: score %.2f%s is below threshold %.2f", dep.Root, policy.Description, s, d, rule.MinimumScore))
			}
		}
	}

	return reasons, nil
}

func loadGoModFile() ([]byte, error) {
	cmd := exec.Command("go", "env", "-json")
	stdout, _ := cmd.StdoutPipe()
	_ = cmd.Start()

	if stdout == nil {
		return os.ReadFile(goModFilename)
	}

	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(stdout)

	goEnv := make(map[string]string)

	err := json.Unmarshal(buf.Bytes(), &goEnv)
	if err != nil {
		return os.ReadFile(goModFilename)
	}

	if _, ok := goEnv["GOMOD"]; !ok {
		return os.ReadFile(goModFilename)
	}

	if _, err = os.Stat(goEnv["GOMOD"]); os.IsNotExist(err) {
		return os.ReadFile(goModFilename)
	}

	if goEnv["GOMOD"] == "/dev/null" {
		return nil, errors.New("current working directory must have a go.mod file")
	}

	return os.ReadFile(goEnv["GOMOD"])
}

// Issue represents the result of one error.
type Issue struct {
	FileName   string
	LineNumber int
	Position   token.Position
	Reason     string
}

// String returns the filename, line
// number and reason of a Issue.
func (r *Issue) String() string {
	return fmt.Sprintf("%s:%d:1 %s", r.FileName, r.LineNumber, r.Reason)
}
