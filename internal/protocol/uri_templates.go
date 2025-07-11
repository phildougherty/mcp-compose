// internal/protocol/uri_templates.go
package protocol

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"mcpcompose/internal/constants"
)

// URITemplate represents an RFC 6570 URI template
type URITemplate struct {
	Template    string
	Expressions []TemplateExpression
}

// TemplateExpression represents a single template expression
type TemplateExpression struct {
	Operator  string         // "", "+", "#", ".", "/", ";", "?", "&"
	Variables []VariableSpec // Variable specifications
	Raw       string         // Raw expression text
	StartPos  int            // Position in template
	EndPos    int            // End position in template
}

// VariableSpec represents a variable specification
type VariableSpec struct {
	Name      string
	Modifier  string // ":" for prefix, "*" for explode
	MaxLength int    // For prefix modifier
}

// ResourceTemplate represents a resource with URI template support
type ResourceTemplate struct {
	URI         string                 `json:"uri"`         // Template URI
	Name        string                 `json:"name"`        // Resource name template
	Description string                 `json:"description"` // Description template
	MimeType    string                 `json:"mimeType,omitempty"`
	Annotations map[string]interface{} `json:"annotations,omitempty"`
	// Template-specific fields
	Variables []TemplateVariable `json:"variables,omitempty"` // Template variables
	Examples  []TemplateExample  `json:"examples,omitempty"`  // Usage examples
}

// TemplateVariable describes a template variable
type TemplateVariable struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Type        string      `json:"type,omitempty"`    // "string", "number", "boolean"
	Default     interface{} `json:"default,omitempty"` // Default value
	Required    bool        `json:"required,omitempty"`
	Pattern     string      `json:"pattern,omitempty"` // Regex pattern for validation
	Enum        []string    `json:"enum,omitempty"`    // Allowed values
}

// TemplateExample shows how to use the template
type TemplateExample struct {
	Description string                 `json:"description,omitempty"`
	Variables   map[string]interface{} `json:"variables"`
	Result      string                 `json:"result"` // Expected URI result
}

// ParseURITemplate parses an RFC 6570 URI template
func ParseURITemplate(template string) (*URITemplate, error) {
	ut := &URITemplate{
		Template: template,
	}

	// Find all template expressions
	expressionRegex := regexp.MustCompile(`\{([^}]+)\}`)
	matches := expressionRegex.FindAllStringSubmatch(template, -1)
	positions := expressionRegex.FindAllStringIndex(template, -1)

	for i, match := range matches {
		if len(match) < constants.MinMatchParts {
			continue
		}

		expr, err := parseExpression(match[1])
		if err != nil {

			return nil, fmt.Errorf("invalid template expression '%s': %w", match[1], err)
		}

		expr.Raw = match[0]
		if len(positions) > i {
			expr.StartPos = positions[i][0]
			expr.EndPos = positions[i][1]
		}

		ut.Expressions = append(ut.Expressions, expr)
	}

	return ut, nil
}

// parseExpression parses a single template expression
func parseExpression(expr string) (TemplateExpression, error) {
	te := TemplateExpression{}

	if len(expr) == 0 {

		return te, fmt.Errorf("empty expression")
	}

	// Check for operator
	operators := []string{"+", "#", ".", "/", ";", "?", "&"}
	for _, op := range operators {
		if strings.HasPrefix(expr, op) {
			te.Operator = op
			expr = expr[1:]

			break
		}
	}

	// Parse variable list
	if err := parseVariableList(expr, &te); err != nil {

		return te, err
	}

	return te, nil
}

// parseVariableList parses a comma-separated list of variables
func parseVariableList(varList string, te *TemplateExpression) error {
	if varList == "" {

		return fmt.Errorf("empty variable list")
	}

	variables := strings.Split(varList, ",")
	for _, variable := range variables {
		varSpec, err := parseVariableSpec(strings.TrimSpace(variable))
		if err != nil {

			return err
		}
		te.Variables = append(te.Variables, varSpec)
	}

	return nil
}

// parseVariableSpec parses a single variable specification
func parseVariableSpec(varSpec string) (VariableSpec, error) {
	vs := VariableSpec{}

	if varSpec == "" {

		return vs, fmt.Errorf("empty variable specification")
	}

	// Check for explode modifier (*)
	if strings.HasSuffix(varSpec, "*") {
		vs.Modifier = "*"
		varSpec = varSpec[:len(varSpec)-1]
	} else if idx := strings.Index(varSpec, ":"); idx != -1 {
		// Check for prefix modifier (:digits)
		vs.Modifier = ":"
		vs.Name = varSpec[:idx]

		lengthStr := varSpec[idx+1:]
		maxLength, err := strconv.Atoi(lengthStr)
		if err != nil {

			return vs, fmt.Errorf("invalid prefix length '%s': %w", lengthStr, err)
		}

		if maxLength <= 0 || maxLength > 10000 {

			return vs, fmt.Errorf("prefix length must be between 1 and 10000")
		}

		vs.MaxLength = maxLength

		return vs, nil
	}

	// Validate variable name
	if !isValidVariableName(varSpec) {

		return vs, fmt.Errorf("invalid variable name '%s'", varSpec)
	}

	vs.Name = varSpec

	return vs, nil
}

// isValidVariableName checks if a variable name is valid per RFC 6570
func isValidVariableName(name string) bool {
	if len(name) == 0 {

		return false
	}

	for i, r := range name {
		if i == 0 {
			// First character must be letter, digit, or underscore
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' {

				return false
			}
		} else {
			// Subsequent characters can be letter, digit, underscore, or dot
			if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '.' {

				return false
			}
		}
	}

	return true
}

// Expand expands the URI template with the given variables
func (ut *URITemplate) Expand(variables map[string]interface{}) (string, error) {
	result := ut.Template

	// Process expressions in reverse order to maintain positions
	for i := len(ut.Expressions) - 1; i >= 0; i-- {
		expr := ut.Expressions[i]
		expansion, err := ut.expandExpression(expr, variables)
		if err != nil {

			return "", fmt.Errorf("failed to expand expression '%s': %w", expr.Raw, err)
		}

		// Replace the expression in the template
		result = result[:expr.StartPos] + expansion + result[expr.EndPos:]
	}

	return result, nil
}

// expandExpression expands a single template expression
func (ut *URITemplate) expandExpression(expr TemplateExpression, variables map[string]interface{}) (string, error) {
	switch expr.Operator {
	case "":

		return ut.expandSimple(expr.Variables, variables)
	case "+":

		return ut.expandReserved(expr.Variables, variables)
	case "#":

		return ut.expandFragment(expr.Variables, variables)
	case ".":

		return ut.expandDot(expr.Variables, variables)
	case "/":

		return ut.expandSlash(expr.Variables, variables)
	case ";":

		return ut.expandSemicolon(expr.Variables, variables)
	case "?":

		return ut.expandQuery(expr.Variables, variables)
	case "&":

		return ut.expandQueryAmp(expr.Variables, variables)
	default:

		return "", fmt.Errorf("unsupported operator: %s", expr.Operator)
	}
}

// expandSimple handles simple string expansion
func (ut *URITemplate) expandSimple(vars []VariableSpec, variables map[string]interface{}) (string, error) {
	var parts []string

	for _, varSpec := range vars {
		value, exists := variables[varSpec.Name]
		if !exists {
			continue
		}

		strValue := ut.valueToString(value)
		if strValue == "" {
			continue
		}

		// Apply modifiers
		if varSpec.Modifier == ":" && varSpec.MaxLength > 0 {
			if len(strValue) > varSpec.MaxLength {
				strValue = strValue[:varSpec.MaxLength]
			}
		}

		// URL encode
		encoded := url.QueryEscape(strValue)
		parts = append(parts, encoded)
	}

	return strings.Join(parts, ","), nil
}

// expandReserved handles reserved string expansion (+)
func (ut *URITemplate) expandReserved(vars []VariableSpec, variables map[string]interface{}) (string, error) {
	var parts []string

	for _, varSpec := range vars {
		value, exists := variables[varSpec.Name]
		if !exists {
			continue
		}

		strValue := ut.valueToString(value)
		if strValue == "" {
			continue
		}

		// Apply modifiers
		if varSpec.Modifier == ":" && varSpec.MaxLength > 0 {
			if len(strValue) > varSpec.MaxLength {
				strValue = strValue[:varSpec.MaxLength]
			}
		}

		// For reserved expansion, don't encode reserved characters
		encoded := ut.encodeReserved(strValue)
		parts = append(parts, encoded)
	}

	return strings.Join(parts, ","), nil
}

// expandFragment handles fragment expansion (#)
func (ut *URITemplate) expandFragment(vars []VariableSpec, variables map[string]interface{}) (string, error) {
	expansion, err := ut.expandReserved(vars, variables)
	if err != nil {

		return "", err
	}

	if expansion == "" {

		return "", nil
	}

	return "#" + expansion, nil
}

// expandDot handles dot expansion (.)
func (ut *URITemplate) expandDot(vars []VariableSpec, variables map[string]interface{}) (string, error) {
	expansion, err := ut.expandSimple(vars, variables)
	if err != nil {

		return "", err
	}

	if expansion == "" {

		return "", nil
	}

	return "." + expansion, nil
}

// expandSlash handles slash expansion (/)
func (ut *URITemplate) expandSlash(vars []VariableSpec, variables map[string]interface{}) (string, error) {
	var parts []string

	for _, varSpec := range vars {
		value, exists := variables[varSpec.Name]
		if !exists {
			continue
		}

		strValue := ut.valueToString(value)
		if strValue == "" {
			continue
		}

		// Apply modifiers
		if varSpec.Modifier == ":" && varSpec.MaxLength > 0 {
			if len(strValue) > varSpec.MaxLength {
				strValue = strValue[:varSpec.MaxLength]
			}
		}

		encoded := url.QueryEscape(strValue)
		parts = append(parts, encoded)
	}

	if len(parts) == 0 {

		return "", nil
	}

	return "/" + strings.Join(parts, "/"), nil
}

// expandSemicolon handles semicolon expansion (;)
func (ut *URITemplate) expandSemicolon(vars []VariableSpec, variables map[string]interface{}) (string, error) {
	var parts []string

	for _, varSpec := range vars {
		value, exists := variables[varSpec.Name]
		if !exists {
			continue
		}

		strValue := ut.valueToString(value)

		// Apply modifiers
		if varSpec.Modifier == ":" && varSpec.MaxLength > 0 {
			if len(strValue) > varSpec.MaxLength {
				strValue = strValue[:varSpec.MaxLength]
			}
		}

		if strValue == "" {
			parts = append(parts, varSpec.Name)
		} else {
			encoded := url.QueryEscape(strValue)
			parts = append(parts, varSpec.Name+"="+encoded)
		}
	}

	if len(parts) == 0 {

		return "", nil
	}

	return ";" + strings.Join(parts, ";"), nil
}

// expandQuery handles query expansion (?)
func (ut *URITemplate) expandQuery(vars []VariableSpec, variables map[string]interface{}) (string, error) {
	var parts []string

	for _, varSpec := range vars {
		value, exists := variables[varSpec.Name]
		if !exists {
			continue
		}

		strValue := ut.valueToString(value)

		// Apply modifiers
		if varSpec.Modifier == ":" && varSpec.MaxLength > 0 {
			if len(strValue) > varSpec.MaxLength {
				strValue = strValue[:varSpec.MaxLength]
			}
		}

		if strValue == "" {
			parts = append(parts, varSpec.Name+"=")
		} else {
			encoded := url.QueryEscape(strValue)
			parts = append(parts, varSpec.Name+"="+encoded)
		}
	}

	if len(parts) == 0 {

		return "", nil
	}

	return "?" + strings.Join(parts, "&"), nil
}

// expandQueryAmp handles query continuation expansion (&)
func (ut *URITemplate) expandQueryAmp(vars []VariableSpec, variables map[string]interface{}) (string, error) {
	expansion, err := ut.expandQuery(vars, variables)
	if err != nil {

		return "", err
	}

	if expansion == "" {

		return "", nil
	}

	// Replace leading ? with &

	return "&" + expansion[1:], nil
}

// valueToString converts a variable value to string
func (ut *URITemplate) valueToString(value interface{}) string {
	if value == nil {

		return ""
	}

	switch v := value.(type) {
	case string:

		return v
	case int:

		return strconv.Itoa(v)
	case int64:

		return strconv.FormatInt(v, 10)
	case float64:

		return strconv.FormatFloat(v, 'f', -1, 64)
	case bool:

		return strconv.FormatBool(v)
	default:

		return fmt.Sprintf("%v", v)
	}
}

// encodeReserved encodes a string but preserves reserved characters
func (ut *URITemplate) encodeReserved(s string) string {
	// Reserved characters that should not be encoded in reserved expansion
	reserved := ":/?#[]@!$&'()*+,;="

	encoded := ""
	for _, r := range s {
		char := string(r)
		if strings.Contains(reserved, char) {
			encoded += char
		} else {
			encoded += url.QueryEscape(char)
		}
	}

	return encoded
}

// GetVariableNames returns all variable names used in the template
func (ut *URITemplate) GetVariableNames() []string {
	var names []string
	seen := make(map[string]bool)

	for _, expr := range ut.Expressions {
		for _, varSpec := range expr.Variables {
			if !seen[varSpec.Name] {
				names = append(names, varSpec.Name)
				seen[varSpec.Name] = true
			}
		}
	}

	return names
}

// Validate validates the template syntax
func (ut *URITemplate) Validate() error {
	// Check for basic syntax errors
	openBraces := strings.Count(ut.Template, "{")
	closeBraces := strings.Count(ut.Template, "}")

	if openBraces != closeBraces {

		return fmt.Errorf("mismatched braces: %d open, %d close", openBraces, closeBraces)
	}

	// Validate each expression
	for _, expr := range ut.Expressions {
		if len(expr.Variables) == 0 {

			return fmt.Errorf("expression '%s' has no variables", expr.Raw)
		}

		for _, varSpec := range expr.Variables {
			if varSpec.Name == "" {

				return fmt.Errorf("empty variable name in expression '%s'", expr.Raw)
			}
		}
	}

	return nil
}
