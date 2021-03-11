package config

import (
	"gopkg.in/yaml.v2"
)

// YAMLParser parse YAML format
type YAMLParser struct {
	content string
}

// Parse convert string into config object
func (parser YAMLParser) Parse(config interface{}) error {
	err := yaml.Unmarshal([]byte(parser.content), config)
	if err != nil {
		return err
	}
	return nil
}

// NewYAMLParser create a YAMLParser
func NewYAMLParser(content string) *YAMLParser {
	parser := new(YAMLParser)
	parser.content = content
	return parser
}
