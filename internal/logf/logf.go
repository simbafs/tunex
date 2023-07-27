package logf

import "log"

func New(name string) func(format string, args ...interface{}) {
	return func(format string, args ...interface{}) {
		log.Printf(name+": "+format, args...)
	}
}
