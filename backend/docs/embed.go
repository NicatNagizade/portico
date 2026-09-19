package docs

import (
	_ "embed"

	"github.com/swaggo/swag"
)

//go:embed swagger.json
var SwaggerJSON []byte

type spec struct{}

func (spec) ReadDoc() string {
	return string(SwaggerJSON)
}

func init() {
	swag.Register(swag.Name, spec{})
}
