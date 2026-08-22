package scheduler

func windowsCommandLine(args []string) string {
	result := ""
	for index, argument := range args {
		if index != 0 {
			result += " "
		}
		result += quoteWindowsArgument(argument)
	}
	return result
}

func quoteWindowsArgument(argument string) string {
	if argument != "" && !containsWindowsArgumentWhitespace(argument) {
		return argument
	}
	result := `"`
	backslashes := 0
	for _, runeValue := range argument {
		if runeValue == '\\' {
			backslashes++
			continue
		}
		if runeValue == '"' {
			result += repeatBackslash(backslashes*2+1) + `"`
			backslashes = 0
			continue
		}
		result += repeatBackslash(backslashes) + string(runeValue)
		backslashes = 0
	}
	return result + repeatBackslash(backslashes*2) + `"`
}

func containsWindowsArgumentWhitespace(value string) bool {
	for _, r := range value {
		if r == ' ' || r == '\t' {
			return true
		}
	}
	return false
}

func repeatBackslash(count int) string {
	result := ""
	for ; count > 0; count-- {
		result += `\`
	}
	return result
}
