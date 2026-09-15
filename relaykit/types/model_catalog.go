package types

// ModelArchitecture is the stable OpenRouter-compatible subset exposed by the
// model catalog. Modality is a compact, derived summary; the input and output
// lists are the authoritative fields.
type ModelArchitecture struct {
	Modality         string   `json:"modality"`
	InputModalities  []string `json:"input_modalities"`
	OutputModalities []string `json:"output_modalities"`
}

// CloneModelArchitecture returns a detached copy that callers may safely
// mutate without changing cached catalog metadata.
func CloneModelArchitecture(value *ModelArchitecture) *ModelArchitecture {
	if value == nil {
		return nil
	}
	return &ModelArchitecture{
		Modality:         value.Modality,
		InputModalities:  append([]string(nil), value.InputModalities...),
		OutputModalities: append([]string(nil), value.OutputModalities...),
	}
}
