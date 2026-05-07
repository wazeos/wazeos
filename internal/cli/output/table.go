package output

import (
	"bytes"
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/fatih/color"

	"github.com/wazeos/wazeos/internal/types"
)

// TableFormatter formats output as ASCII tables
type TableFormatter struct {
	NoColor bool
}

func (f *TableFormatter) FormatPackageList(apps []*types.AppMetadata) (string, error) {
	if len(apps) == 0 {
		return f.FormatSuccess("No packages installed"), nil
	}

	buf := &bytes.Buffer{}

	// Use fixed-width formatting instead of tabwriter to avoid ANSI color code issues
	// Column widths: ID(50) DESCRIPTION(remaining)

	// Print header
	if !f.NoColor {
		// Manually pad colored headers to account for ANSI codes
		idHeader := color.New(color.FgCyan, color.Bold).Sprint("ID") + strings.Repeat(" ", 48)      // 50 - 2
		descHeader := color.New(color.FgCyan, color.Bold).Sprint("DESCRIPTION")

		buf.WriteString(fmt.Sprintf("%s %s\n", idHeader, descHeader))
	} else {
		fmt.Fprintf(buf, "%-50s %s\n", "ID", "DESCRIPTION")
	}

	// Print rows
	for _, app := range apps {
		desc := app.Description
		if len(desc) > 80 {
			desc = desc[:77] + "..."
		}

		// Get app ID (author/name_version)
		appID := app.AppID()
		if len(appID) > 50 {
			appID = appID[:47] + "..."
		}

		// Format row with fixed widths
		if !f.NoColor {
			coloredID := color.New(color.FgGreen).Sprint(appID)

			// Calculate padding to account for ANSI codes
			idPad := 50 - len(appID)

			row := fmt.Sprintf("%s%s %s\n",
				coloredID, strings.Repeat(" ", idPad),
				desc)
			buf.WriteString(row)
		} else {
			fmt.Fprintf(buf, "%-50s %s\n", appID, desc)
		}
	}

	// Add count footer
	footer := fmt.Sprintf("\nTotal: %d package(s)", len(apps))
	if !f.NoColor {
		footer = color.New(color.FgHiBlack).Sprint(footer)
	}
	buf.WriteString(footer)

	return buf.String(), nil
}

func (f *TableFormatter) FormatPackageDetails(app *types.AppMetadata) (string, error) {
	buf := &bytes.Buffer{}

	// Title
	title := fmt.Sprintf("Package: %s", app.Name)
	if !f.NoColor {
		title = color.New(color.FgCyan, color.Bold).Sprint(title)
	}
	buf.WriteString(title + "\n")
	buf.WriteString(strings.Repeat("-", len("Package: "+app.Name)) + "\n\n")

	// Details
	details := []struct {
		label string
		value string
	}{
		{"Name", app.Name},
		{"Version", app.Version},
		{"Author", app.Author},
		{"Type", string(app.Type)},
		{"Description", app.Description},
	}

	if app.DriverClass != "" {
		details = append(details, struct{ label, value string }{"Driver Class", app.DriverClass})
	}

	if app.DependenciesV2 != nil && (len(app.DependenciesV2.Apps) > 0 || len(app.DependenciesV2.Drivers) > 0) {
		var deps []string
		for name, ver := range app.DependenciesV2.Apps {
			deps = append(deps, fmt.Sprintf("app:%s@%s", name, ver))
		}
		for name, ver := range app.DependenciesV2.Drivers {
			deps = append(deps, fmt.Sprintf("driver:%s@%s", name, ver))
		}
		details = append(details, struct{ label, value string }{"Dependencies", strings.Join(deps, ", ")})
	}

	if app.PrerequisitesV2 != nil && (len(app.PrerequisitesV2.Apps) > 0 || len(app.PrerequisitesV2.Drivers) > 0) {
		var prereqs []string
		for name, ver := range app.PrerequisitesV2.Apps {
			prereqs = append(prereqs, fmt.Sprintf("app:%s@%s", name, ver))
		}
		for name, ver := range app.PrerequisitesV2.Drivers {
			prereqs = append(prereqs, fmt.Sprintf("driver:%s@%s", name, ver))
		}
		details = append(details, struct{ label, value string }{"Prerequisites", strings.Join(prereqs, ", ")})
	}

	// Print details
	for _, d := range details {
		label := d.label + ":"
		if !f.NoColor {
			label = color.New(color.FgYellow).Sprint(label)
		}
		buf.WriteString(fmt.Sprintf("%-15s %s\n", label, d.value))
	}

	// Input schema if present
	if app.InputSchema != nil {
		buf.WriteString("\n")
		schemaLabel := "Input Schema:"
		if !f.NoColor {
			schemaLabel = color.New(color.FgYellow).Sprint(schemaLabel)
		}
		buf.WriteString(schemaLabel + "\n")
		buf.WriteString(string(*app.InputSchema) + "\n")
	}

	return buf.String(), nil
}

func (f *TableFormatter) FormatSecretList(keys []string) (string, error) {
	if len(keys) == 0 {
		return f.FormatSuccess("No secrets found"), nil
	}

	buf := &bytes.Buffer{}
	w := tabwriter.NewWriter(buf, 0, 0, 2, ' ', 0)

	// Print header
	if !f.NoColor {
		fmt.Fprintf(w, "%s\n", color.New(color.FgCyan, color.Bold).Sprint("SECRET KEY"))
	} else {
		fmt.Fprintf(w, "SECRET KEY\n")
	}

	// Print rows
	for _, key := range keys {
		keyStr := key
		if !f.NoColor {
			keyStr = color.New(color.FgGreen).Sprint(keyStr)
		}
		fmt.Fprintf(w, "%s\n", keyStr)
	}

	w.Flush()

	// Add count footer
	footer := fmt.Sprintf("\nTotal: %d secret(s)", len(keys))
	if !f.NoColor {
		footer = color.New(color.FgHiBlack).Sprint(footer)
	}
	buf.WriteString(footer)

	return buf.String(), nil
}

func (f *TableFormatter) FormatError(err error) string {
	symbol := "✗"
	message := fmt.Sprintf("%s Error: %s", symbol, err.Error())
	if !f.NoColor {
		return color.New(color.FgRed, color.Bold).Sprint(message)
	}
	return message
}

func (f *TableFormatter) FormatSuccess(message string) string {
	symbol := "✓"
	msg := fmt.Sprintf("%s %s", symbol, message)
	if !f.NoColor {
		return color.New(color.FgGreen).Sprint(msg)
	}
	return msg
}
