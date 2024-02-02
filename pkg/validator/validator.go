// Package validator extends validator.Validate with regex and few other validation capabilities.
package validator

import (
	"log"
	"regexp"

	"github.com/go-playground/validator/v10"
)

// Validate is a custom validator that extends the base validator.Validate.
type Validate struct {
	validator.Validate
}

// New creates a new instance of Validate
func New() *Validate {
	validate := &Validate{
		Validate: *validator.New(),
	}

	err := validate.RegisterValidation("regex", validateRegex)
	if err != nil {
		log.Fatalf("failed to register regex validator: %s", err)
	}

	return validate
}

// validateRegex is the custom validation function that checks if the field value
// matches the provided regular expression.
func validateRegex(fl validator.FieldLevel) bool {
	field := fl.Field()
	regexTag := fl.Param()

	regex := regexp.MustCompile(regexTag)

	return regex.MatchString(field.String())
}
