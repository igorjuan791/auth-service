package main

import (
	"context"
	"errors"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// initTelemetry configura traces e métricas.
// O endereço do Collector vem da variável de ambiente OTEL_EXPORTER_OTLP_ENDPOINT.
func initTelemetry(ctx context.Context, defaultName string) (func(context.Context) error, error) {
	// 1) Identidade do serviço (nome, host, etc.)
	res, err := resource.New(ctx,
		resource.WithAttributes(attribute.String("service.name", defaultName)),
		resource.WithFromEnv(), // OTEL_SERVICE_NAME, se existir, sobrescreve o nome acima
		resource.WithTelemetrySDK(),
		resource.WithHost(),
	)
	if err != nil {
		return nil, err
	}

	// 2) Traces: criados pelo serviço e enviados em lotes ao Collector
	traceExp, err := otlptracegrpc.New(ctx)
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(traceExp),
		sdktrace.WithResource(res),
	)

	// 3) Métricas: enviadas ao Collector a cada 15 segundos
	metricExp, err := otlpmetricgrpc.New(ctx)
	if err != nil {
		return nil, err
	}
	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(sdkmetric.NewPeriodicReader(metricExp, sdkmetric.WithInterval(15*time.Second))),
		sdkmetric.WithResource(res),
	)

	// 4) Registra tudo como padrão global e define o formato de propagação do trace
	otel.SetTracerProvider(tp)
	otel.SetMeterProvider(mp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))

	// 5) Devolve uma função para "fechar" tudo ao desligar o serviço
	return func(ctx context.Context) error {
		return errors.Join(tp.Shutdown(ctx), mp.Shutdown(ctx))
	}, nil
}