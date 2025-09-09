package server

import (
	"bufio"
	"encoding/base64"
	"fmt"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/rs/zerolog/log"
)

func b64Encode(input string) string {
	return base64.StdEncoding.EncodeToString([]byte(input))
}

func trim(input string) string {
	return strings.TrimSpace(input)
}

func readFile(filePath string) (string, error) {
	resolvedFile := os.ExpandEnv(filePath)
	log.Trace().Msgf("filter:readFile %s", resolvedFile)
	content, err := os.ReadFile(resolvedFile)
	if err != nil {
		return "", err
	}
	return string(content), nil
}

// readNthLine reads the nth line of a file.
func readNthLine(n int, filePath string) (string, error) {
	log.Trace().Msgf("filter:readNthLine[%d] %s", n, filePath)

	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for i := 0; scanner.Scan(); i++ {
		if i == n-1 {
			return scanner.Text(), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", nil
}

func readEnv(envName string) string {
	return os.Getenv(envName)
}

func concat(input, suffix string) string {
	return input + suffix
}

func expandEnv(inputStr string) string {
	return os.ExpandEnv(inputStr)
}

func strJoin(input []interface{}, separators ...string) string {
	separator := ","
	if len(separators) > 0 && separators[0] != "" {
		separator = separators[0]
	}

	strList := make([]string, 0, len(input))
	for _, item := range input {
		strList = append(strList, fmt.Sprintf("%v", item))
	}
	return strings.Join(strList, separator)
}

func renderAllTemplates(content string, mappings map[string]interface{}, params ConfigParams) string {
	pattern := fmt.Sprintf(`%s[^\n]*?%s`, regexp.QuoteMeta(params.LeftDelim), regexp.QuoteMeta(params.RightDelim))
	log.Debug().Msgf("template-pattern: %s", pattern)
	re := regexp.MustCompile(pattern)

	matches := re.FindAllStringIndex(content, -1)

	result := content
	offset := 0

	for _, match := range matches {
		start, end := match[0]+offset, match[1]+offset
		originalMatch := result[start:end]
		replacement := tryRenderTemplate(originalMatch, mappings, params)
		result = result[:start] + replacement + result[end:]

		offset += len(replacement) - (end - start)
	}

	return result
}

func unescapeYAMLTemplate(input string) string {
	// Unescape YAML double-quoted scalar escapes so template parser sees valid Go strings
	input = strings.ReplaceAll(input, `\"`, `"`)
	input = strings.ReplaceAll(input, `\\`, `\`)
	return input
}

func tryRenderTemplate(input string, context map[string]interface{}, params ConfigParams) string {
	processed := unescapeYAMLTemplate(input)

	tmpl, err := template.New("config").Delims(params.LeftDelim, params.RightDelim).Funcs(template.FuncMap{
		"b64encode":   b64Encode,
		"concat":      concat,
		"expandEnv":   expandEnv,
		"env":         readEnv,
		"readFile":    readFile,
		"readNthLine": readNthLine,
		"strJoin":     strJoin,
		"trim":        trim,
	}).Parse(processed)
	if err != nil {
		log.Err(err).Msgf("bad configuration template, %s", input)
		return input
	}

	var rendered strings.Builder
	if err := tmpl.Execute(&rendered, context); err != nil {
		log.Trace().Msgf("[render failed]. input=%s, context=%v, err=%v", input, context, err)
		return input
	}

	result := rendered.String()
	log.Trace().Msgf("[render ok] %s => %s", input, SecureLogKey(result))
	return result
}
