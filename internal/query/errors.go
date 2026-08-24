package query

import "fmt"

// Error é um erro de consulta que a API consegue transformar em uma resposta
// 400 útil. ValidFields só é preenchido quando o nome de um campo não existe.
type Error struct {
	Parameter   string   `json:"parameter,omitempty"`
	Message     string   `json:"message"`
	ValidFields []string `json:"valid_fields,omitempty"`
}

func (e *Error) Error() string {
	if e == nil {
		return "consulta inválida"
	}
	if len(e.ValidFields) == 0 {
		return e.Message
	}
	return fmt.Sprintf("%s; campos válidos: %s", e.Message, joinComma(e.ValidFields))
}

func joinComma(values []string) string {
	result := ""
	for i, value := range values {
		if i > 0 {
			result += ", "
		}
		result += value
	}
	return result
}

func invalid(parameter, message string) error {
	return &Error{Parameter: parameter, Message: message}
}
