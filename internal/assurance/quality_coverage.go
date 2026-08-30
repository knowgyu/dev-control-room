package assurance

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"
)

const qualityCoverageMaxProfileBytes = 1 << 20

// GoCoverageSummary is the reviewed, bounded subset of a Go coverprofile that
// is persisted as structured QualityRun evidence.
type GoCoverageSummary struct {
	Mode              string  `json:"mode"`
	FileCount         int     `json:"fileCount"`
	TotalStatements   int     `json:"totalStatements"`
	CoveredStatements int     `json:"coveredStatements"`
	Percent           float64 `json:"percent"`
}

// ParseGoCoverageProfile parses the native Go coverprofile text format without
// invoking a shell or trusting any executable-produced JSON.
func ParseGoCoverageProfile(data []byte) (GoCoverageSummary, error) {
	if len(data) == 0 || len(data) > qualityCoverageMaxProfileBytes {
		return GoCoverageSummary{}, errors.New("go coverage profile is empty or exceeds its fixed bound")
	}
	if !utf8.Valid(data) {
		return GoCoverageSummary{}, errors.New("go coverage profile is not valid UTF-8")
	}

	var summary GoCoverageSummary
	files := make(map[string]struct{})
	headerSeen := false
	recordSeen := false
	for lineNumber, rawLine := range bytes.Split(data, []byte{'\n'}) {
		line := strings.TrimSpace(string(rawLine))
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "mode:") {
			if headerSeen || recordSeen {
				return GoCoverageSummary{}, fmt.Errorf("go coverage profile has an unexpected mode line at line %d", lineNumber+1)
			}
			fields := strings.Fields(line)
			if len(fields) != 2 || fields[0] != "mode:" || !validGoCoverageMode(fields[1]) {
				return GoCoverageSummary{}, fmt.Errorf("go coverage profile mode is invalid at line %d", lineNumber+1)
			}
			summary.Mode = fields[1]
			headerSeen = true
			continue
		}
		if !headerSeen {
			return GoCoverageSummary{}, fmt.Errorf("go coverage profile is missing its mode at line %d", lineNumber+1)
		}
		recordSeen = true
		location, statementText, countText, ok := splitGoCoverageRecord(line)
		if !ok {
			return GoCoverageSummary{}, fmt.Errorf("go coverage profile record is malformed at line %d", lineNumber+1)
		}
		statements, err := strconv.Atoi(statementText)
		if err != nil || statements <= 0 {
			return GoCoverageSummary{}, fmt.Errorf("go coverage profile statement count is invalid at line %d", lineNumber+1)
		}
		count, err := strconv.Atoi(countText)
		if err != nil || count < 0 {
			return GoCoverageSummary{}, fmt.Errorf("go coverage profile execution count is invalid at line %d", lineNumber+1)
		}
		if !validGoCoverageLocation(location) {
			return GoCoverageSummary{}, fmt.Errorf("go coverage profile location is invalid at line %d", lineNumber+1)
		}
		files[locationFile(location)] = struct{}{}
		summary.TotalStatements += statements
		if count > 0 {
			summary.CoveredStatements += statements
		}
	}
	if !headerSeen {
		return GoCoverageSummary{}, errors.New("go coverage profile is missing its mode")
	}
	summary.FileCount = len(files)
	if summary.TotalStatements > 0 {
		summary.Percent = float64(summary.CoveredStatements) * 100 / float64(summary.TotalStatements)
	}
	return summary, nil
}

func validGoCoverageMode(value string) bool {
	switch value {
	case "set", "count", "atomic":
		return true
	default:
		return false
	}
}

func splitGoCoverageRecord(line string) (location, statements, count string, ok bool) {
	last := strings.LastIndexByte(line, ' ')
	if last < 0 {
		return "", "", "", false
	}
	secondLast := strings.LastIndexByte(line[:last], ' ')
	if secondLast < 0 {
		return "", "", "", false
	}
	location = strings.TrimSpace(line[:secondLast])
	statements = strings.TrimSpace(line[secondLast:last])
	count = strings.TrimSpace(line[last+1:])
	return location, statements, count, location != "" && statements != "" && count != ""
}

func validGoCoverageLocation(value string) bool {
	separator := strings.LastIndexByte(value, ':')
	if separator <= 0 || separator == len(value)-1 {
		return false
	}
	startEnd := strings.Split(value[separator+1:], ",")
	if len(startEnd) != 2 {
		return false
	}
	for _, point := range startEnd {
		parts := strings.Split(point, ".")
		if len(parts) != 2 {
			return false
		}
		line, lineErr := strconv.Atoi(parts[0])
		column, columnErr := strconv.Atoi(parts[1])
		if lineErr != nil || columnErr != nil || line <= 0 || column <= 0 {
			return false
		}
	}
	return locationFile(value) != ""
}

func locationFile(value string) string {
	separator := strings.LastIndexByte(value, ':')
	if separator <= 0 {
		return ""
	}
	return value[:separator]
}
