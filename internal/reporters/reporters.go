package reporters

import (
	"fmt"
	"io"

	"github.com/go-xmlfmt/xmlfmt"
	"github.com/phayes/checkstyle"
	"github.com/thatsmrtalbot/goscorecardcheck"
)

type Reporter interface {
	Write(io.Writer, []goscorecardcheck.Issue) error
}

var Checkstyle Reporter = checkstyleReporter{}
var Default Reporter = defaultReporter{}

type checkstyleReporter struct{}

func (checkstyleReporter) Write(w io.Writer, issues []goscorecardcheck.Issue) error {
	// Create report
	check := checkstyle.New()
	for _, issue := range issues {
		file := check.EnsureFile(issue.FileName)
		file.AddError(checkstyle.NewError(issue.LineNumber, 1, checkstyle.SeverityError, issue.Reason, "goscorecardcheck"))
	}

	// Write the XML
	checkstyleXML := fmt.Sprintf("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n%s", check.String())
	_, err := w.Write([]byte(xmlfmt.FormatXML(checkstyleXML, "", "  ")))
	return err
}

type defaultReporter struct{}

func (defaultReporter) Write(w io.Writer, issues []goscorecardcheck.Issue) error {
	for _, issue := range issues {
		fmt.Fprintln(w, issue.String())
	}
	return nil
}
