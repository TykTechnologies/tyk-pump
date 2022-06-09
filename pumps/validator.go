package pumps

type Validator struct {
}

func (v *Validator) Validate(q Query) Result {
	return Result{}
}

type Query interface {
}

type Result struct {
	Data []interface{}
	Err  error
}

func (r *Result) GetRawData() string {
	return ""
}
