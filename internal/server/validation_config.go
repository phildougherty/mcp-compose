package server

import (
	"strconv"
	"strings"

	"github.com/phildougherty/mcp-compose/internal/config"
)

func LoadValidationConfigFromCompose(cfg *config.ComposeConfig) *ValidationConfig {
	valConfig := DefaultValidationConfig()

	if cfg == nil || !cfg.Validation.Enabled {
		return valConfig
	}

	if cfg.Validation.MaxBodySize != "" {
		if size, err := parseByteSize(cfg.Validation.MaxBodySize); err == nil {
			valConfig.MaxBodySize = size
		}
	}

	valConfig.RequireContentType = cfg.Validation.RequireContentType

	if len(cfg.Validation.AllowedContentTypes) > 0 {
		valConfig.AllowedContentTypes = cfg.Validation.AllowedContentTypes
	}

	valConfig.StripHTML = cfg.Validation.StripHTML
	valConfig.ValidateJSON = cfg.Validation.ValidateJSON
	valConfig.RequireJSONRPC = cfg.Validation.RequireJSONRPC

	return valConfig
}

func parseByteSize(s string) (int64, error) {
	s = strings.TrimSpace(strings.ToUpper(s))

	multipliers := map[string]int64{
		"B":  1,
		"KB": 1024,
		"MB": 1024 * 1024,
		"GB": 1024 * 1024 * 1024,
	}

	for suffix, multiplier := range multipliers {
		if strings.HasSuffix(s, suffix) {
			numStr := strings.TrimSuffix(s, suffix)
			numStr = strings.TrimSpace(numStr)

			num, err := strconv.ParseInt(numStr, 10, 64)
			if err != nil {
				return 0, err
			}

			return num * multiplier, nil
		}
	}

	return strconv.ParseInt(s, 10, 64)
}