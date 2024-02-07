package dns

import (
	"bufio"
	"io"
	"strings"

	ss "github.com/localstack-samples/localstack-on-eks/pkg/crds/internal/strings"
)

// DirectiveEntry represents a single entry in a directive block
type DirectiveEntry struct {
	IsMap    bool
	MapValue map[string][]string
	StrValue string
}

// Directive represents a single directive in the Corefile
type Directive struct {
	Name    string
	Params  []string
	Entries []DirectiveEntry
}

// CoreConfig represents the structure of a Corefile configuration
type CoreConfig struct {
	Directives []Directive
}

// CorefileParser defines the interface for parsing and marshaling Corefile configurations
type CorefileParser interface {
	Parse(reader io.Reader) (CoreConfig, error)
	Marshal(config CoreConfig) (string, error)
	Unmarshal(data string) (CoreConfig, error)
}

// DefaultCorefileParser implements CorefileParser using bufio.Scanner
type DefaultCorefileParser struct{}

// Parse parses a Corefile from the given io.Reader
func (p *DefaultCorefileParser) Parse(reader io.Reader) (CoreConfig, error) {
	var config CoreConfig
	scanner := bufio.NewScanner(reader)
	var currentDirective Directive
	var insideEntry bool // Flag to indicate if we are inside a directive entry
	for scanner.Scan() {
		line := scanner.Text()
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check for opening curly brace indicating start of a directive block
		if !insideEntry && strings.HasSuffix(line, "{") {
			// Trim the curly brace and whitespace
			directiveName := strings.TrimSpace(strings.TrimSuffix(line, "{"))
			currentDirective = Directive{Name: directiveName}
			insideEntry = true
		} else if !strings.Contains(line, "{") && strings.HasSuffix(line, "}") {
			// End of directive block, append the directive to config
			config.Directives = append(config.Directives, currentDirective)
			currentDirective = Directive{} // Reset currentDirective
			insideEntry = false
		} else {
			// When it's a single entry under the directive with no curly braces
			if !strings.HasSuffix(line, "{") && !strings.HasSuffix(line, "}") {
				entry := DirectiveEntry{IsMap: false, StrValue: line}
				currentDirective.Entries = append(currentDirective.Entries, entry)
			} else if strings.HasSuffix(line, "{}") { // When it's an empty block under the directive with curly braces
				entry := DirectiveEntry{IsMap: false, StrValue: strings.TrimSpace(line)}
				currentDirective.Entries = append(currentDirective.Entries, entry)
			} else if strings.HasSuffix(line, "}") { // When it's an inline single entry under the directive with curly braces
				entry := DirectiveEntry{IsMap: true, MapValue: make(map[string][]string)}

				// Get value inside curly braces
				value := strings.TrimSpace(ss.RsplitN(strings.TrimSpace(strings.TrimSuffix(line, "}")), "{", 1)[1])

				// Add the key-value pair to the map
				var mapKey, mapValue string
				valueLines := ss.RsplitN(line, " ", 1)
				mapKey = valueLines[0]
				if len(valueLines) == 2 {
					mapValue = valueLines[1]
				}
				entry.StrValue = value
				entry.MapValue[mapKey] = append(entry.MapValue[mapKey], mapValue)
				currentDirective.Entries = append(currentDirective.Entries, entry)
			} else if strings.HasSuffix(line, "{") { // When it's a block under the directive with curly braces
				parts := ss.RsplitN(line, " ", 1)
				entry := DirectiveEntry{IsMap: true, StrValue: parts[0], MapValue: make(map[string][]string)}

				// Start reading the block inside the directive.
				for scanner.Scan() {
					line := scanner.Text()
					line = strings.TrimSpace(line)

					// Skip empty lines and comments
					if line == "" || strings.HasPrefix(line, "#") {
						continue
					}

					// Stop constructing the block when we encounter the closing curly brace
					if !strings.Contains(line, "{") && strings.HasSuffix(line, "}") {
						break
					}

					// Add the key-value pair to the map
					var mapKey, mapValue string
					valueLines := ss.RsplitN(line, " ", 1)
					mapKey = valueLines[0]
					if len(valueLines) == 2 {
						mapValue = valueLines[1]
					}
					entry.MapValue[mapKey] = append(entry.MapValue[mapKey], mapValue)
				}
				currentDirective.Entries = append(currentDirective.Entries, entry)
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return CoreConfig{}, err
	}

	return config, nil
}

// Marshal converts a CoreConfig into a Corefile string
func (p *DefaultCorefileParser) Marshal(config CoreConfig) (string, error) {
	var sb strings.Builder
	for _, directive := range config.Directives {
		sb.WriteString(directive.Name)
		sb.WriteString(" {\n")
		for _, entry := range directive.Entries {
			if entry.IsMap {
				// If entry has a map value, handle indentation and write key-value pairs
				sb.WriteString("\t")
				if entry.StrValue != "" {
					sb.WriteString(entry.StrValue)
					sb.WriteString(" {\n")
				}
				for key, values := range entry.MapValue {
					for _, value := range values {
						sb.WriteString("\t\t")
						sb.WriteString(key)
						sb.WriteString(" ")
						sb.WriteString(value)
						sb.WriteString("\n")
					}
				}
				if entry.StrValue != "" {
					sb.WriteString("\t}\n")
				}
			} else {
				// Handle single string entry directly under the directive
				sb.WriteString("\t")
				sb.WriteString(entry.StrValue)
				sb.WriteString("\n")
			}
		}
		sb.WriteString("}\n\n")
	}
	return sb.String(), nil
}

// Unmarshal parses a Corefile string and returns a CoreConfig
func (p *DefaultCorefileParser) Unmarshal(data string) (CoreConfig, error) {
	// Create a strings reader to parse the data
	reader := strings.NewReader(data)
	return p.Parse(reader)
}

// HasDirective checks if a CoreConfig has a directive with the given name
func (c *CoreConfig) HasDirective(directiveName string) bool {
	for _, directive := range c.Directives {
		if directive.Name == directiveName {
			return true
		}
	}
	return false
}

// AddDirective adds a new directive to the CoreConfig
func (c *CoreConfig) AddDirective(directive Directive) {
	c.Directives = append(c.Directives, directive)
}

// RemoveDirective removes a directive from the CoreConfig.
// It returns the number of directives removed.
func (c *CoreConfig) RemoveDirective(directiveName string) int {
	newDirectiveList := []Directive{}
	removedDirectives := 0
	for i, directive := range c.Directives {
		if directive.Name != directiveName {
			newDirectiveList = append(newDirectiveList, c.Directives[i])
		} else {
			removedDirectives++
		}
	}
	c.Directives = newDirectiveList
	return removedDirectives
}

// KeepUniqueDirectives removes duplicate directives from the CoreConfig.
// It returns the number of duplicate directives removed.
func (c *CoreConfig) KeepUniqueDirectives() int {
	uniqueDirectives := []Directive{}
	seen := map[string]bool{}
	removedDirectives := 0
	for _, directive := range c.Directives {
		if _, ok := seen[directive.Name]; !ok {
			seen[directive.Name] = true
			uniqueDirectives = append(uniqueDirectives, directive)
		} else {
			removedDirectives++
		}
	}
	c.Directives = uniqueDirectives
	return removedDirectives
}

// Get directive names from the CoreConfig
func (c *CoreConfig) GetDirectiveNames() []string {
	var names []string
	for _, directive := range c.Directives {
		names = append(names, directive.Name)
	}
	return names
}
