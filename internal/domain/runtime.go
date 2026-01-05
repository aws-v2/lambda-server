package domain

type Runtime string

const (
	RuntimeJavaScript Runtime = "javascript"
	RuntimeDocker     Runtime = "docker"
)

func (r Runtime) IsValid() bool {
	switch r {
	case RuntimeJavaScript, RuntimeDocker:
		return true
	default:
		return false
	}
}

func (r Runtime) String() string {
	return string(r)
}
