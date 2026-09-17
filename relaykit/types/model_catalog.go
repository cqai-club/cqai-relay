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
	inputModalities := make([]string, len(value.InputModalities))
	copy(inputModalities, value.InputModalities)
	outputModalities := make([]string, len(value.OutputModalities))
	copy(outputModalities, value.OutputModalities)
	return &ModelArchitecture{
		Modality:         value.Modality,
		InputModalities:  inputModalities,
		OutputModalities: outputModalities,
	}
}
