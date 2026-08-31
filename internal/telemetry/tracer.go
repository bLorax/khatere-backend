package telemetry

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

// Tracer returns the application-layer tracer. Every use case starts its
// span from this, so all application spans share one consistent
// instrumentation name in Jaeger.
func Tracer() trace.Tracer {
	return otel.Tracer("yadegar/application")
}
