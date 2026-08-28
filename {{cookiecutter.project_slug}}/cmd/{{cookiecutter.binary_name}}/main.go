// Command {{ cookiecutter.binary_name }} is {{ cookiecutter.project_description|lower }}
//
// Everything it does lives in internal/bootstrap, so that the same entry point
// can be exercised in-process by the functional test suite.
package main

import (
	"context"
	"os"

	"{{ cookiecutter.module_path }}/internal/bootstrap"
)

func main() {
	os.Exit(bootstrap.Run(context.Background(), bootstrap.Options{}))
}
