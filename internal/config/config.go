package config

import (
	"fmt"
	"os"
)

// InterpolateContent interpolate variables from current environment
func InterpolateContent(content *[]byte) *string {
	strContent := string(*content)
	strContent = os.ExpandEnv(strContent)
	return &strContent
}

// ParseContent create a Config from content
func ParseContent(content *string, parser *string, destination interface{}) error {
	// Check parser type
	switch *parser {
	case "json":
		jsonParser := NewJSONParser(*content)
		err := jsonParser.Parse(destination)
		if err != nil {
			return err
		}
		break
	case "yaml":
	case "yml":
		yamlParser := NewYAMLParser(*content)
		err := yamlParser.Parse(destination)
		if err != nil {
			return err
		}
		break
	default:
		return fmt.Errorf("Parser %s not supported", *parser)
	}

	return nil
}
