package coquelicot

type option func(*Storage)

func (s *Storage) Option(opts ...option) {
	for _, opt := range opts {
		opt(s)
	}
}

// Verbosity sets verbosity level (1 to 3).
func Verbosity(level int) option {
	return func(s *Storage) {
		s.verbosity = level
	}
}

// Convert generates an image thumbnail using ImageMagick.
func Convert(b bool) option {
	return func(s *Storage) {
		makeThumbnail = b
	}
}
