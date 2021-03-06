package strategy

import (
	"context"
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"

	"github.com/jaegertracing/jaeger-operator/pkg/apis/jaegertracing/v1"
	"github.com/jaegertracing/jaeger-operator/pkg/cronjob"
	"github.com/jaegertracing/jaeger-operator/pkg/storage"
)

// For returns the appropriate Strategy for the given Jaeger instance
func For(ctx context.Context, jaeger *v1.Jaeger) S {
	if strings.EqualFold(jaeger.Spec.Strategy, "all-in-one") {
		jaeger.Logger().Warn("Strategy 'all-in-one' is no longer supported, please use 'allInOne'")
		jaeger.Spec.Strategy = "allInOne"
	}

	normalize(jaeger)

	jaeger.Logger().WithField("strategy", jaeger.Spec.Strategy).Debug("Strategy chosen")
	if strings.EqualFold(jaeger.Spec.Strategy, "allinone") {
		return newAllInOneStrategy(jaeger)
	}

	if strings.EqualFold(jaeger.Spec.Strategy, "streaming") {
		return newStreamingStrategy(jaeger)
	}

	return newProductionStrategy(jaeger)
}

// normalize changes the incoming Jaeger object so that the defaults are applied when
// needed and incompatible options are cleaned
func normalize(jaeger *v1.Jaeger) {
	// we need a name!
	if jaeger.Name == "" {
		jaeger.Logger().Info("This Jaeger instance was created without a name. Applying a default name.")
		jaeger.Name = "my-jaeger"
	}

	// normalize the storage type
	if jaeger.Spec.Storage.Type == "" {
		jaeger.Logger().Info("Storage type not provided. Falling back to 'memory'")
		jaeger.Spec.Storage.Type = "memory"
	}

	if unknownStorage(jaeger.Spec.Storage.Type) {
		jaeger.Logger().WithFields(log.Fields{
			"storage":       jaeger.Spec.Storage.Type,
			"known-options": storage.ValidTypes(),
		}).Info("The provided storage type is unknown. Falling back to 'memory'")
		jaeger.Spec.Storage.Type = "memory"
	}

	// normalize the deployment strategy
	if !strings.EqualFold(jaeger.Spec.Strategy, "production") && !strings.EqualFold(jaeger.Spec.Strategy, "streaming") {
		jaeger.Spec.Strategy = "allInOne"
	}

	// check for incompatible options
	// if the storage is `memory`, then the only possible strategy is `all-in-one`
	if strings.EqualFold(jaeger.Spec.Storage.Type, "memory") && !strings.EqualFold(jaeger.Spec.Strategy, "allinone") {
		jaeger.Logger().WithField("storage", jaeger.Spec.Storage.Type).Warn("No suitable storage provided. Falling back to all-in-one")
		jaeger.Spec.Strategy = "allInOne"
	}

	// we always set the value to None, except when we are on OpenShift *and* the user has not explicitly set to 'none'
	if viper.GetString("platform") == v1.FlagPlatformOpenShift && jaeger.Spec.Ingress.Security != v1.IngressSecurityNoneExplicit {
		jaeger.Spec.Ingress.Security = v1.IngressSecurityOAuthProxy
	} else {
		// cases:
		// - omitted on Kubernetes
		// - 'none' on any platform
		jaeger.Spec.Ingress.Security = v1.IngressSecurityNone
	}

	normalizeSparkDependencies(&jaeger.Spec.Storage.SparkDependencies, jaeger.Spec.Storage.Type)
	normalizeIndexCleaner(&jaeger.Spec.Storage.EsIndexCleaner, jaeger.Spec.Storage.Type)
	normalizeElasticsearch(&jaeger.Spec.Storage.Elasticsearch)
	normalizeRollover(&jaeger.Spec.Storage.Rollover)
}

func normalizeSparkDependencies(spec *v1.JaegerDependenciesSpec, storage string) {
	// auto enable only for supported storages
	if cronjob.SupportedStorage(storage) && spec.Enabled == nil {
		trueVar := true
		spec.Enabled = &trueVar
	}
	if spec.Image == "" {
		spec.Image = fmt.Sprintf("%s", viper.GetString("jaeger-spark-dependencies-image"))
	}
	if spec.Schedule == "" {
		spec.Schedule = "55 23 * * *"
	}
}

func normalizeIndexCleaner(spec *v1.JaegerEsIndexCleanerSpec, storage string) {
	// auto enable only for supported storages
	if storage == "elasticsearch" && spec.Enabled == nil {
		trueVar := true
		spec.Enabled = &trueVar
	}
	if spec.Image == "" {
		spec.Image = fmt.Sprintf("%s", viper.GetString("jaeger-es-index-cleaner-image"))
	}
	if spec.Schedule == "" {
		spec.Schedule = "55 23 * * *"
	}
	if spec.NumberOfDays == 0 {
		spec.NumberOfDays = 7
	}
}

func normalizeElasticsearch(spec *v1.ElasticsearchSpec) {
	if spec.NodeCount == 0 {
		spec.NodeCount = 1
	}
}

func normalizeRollover(spec *v1.JaegerEsRolloverSpec) {
	if spec.Image == "" {
		spec.Image = fmt.Sprintf("%s", viper.GetString("jaeger-es-rollover-image"))
	}
	if spec.Schedule == "" {
		spec.Schedule = "*/30 * * * *"
	}
}

func unknownStorage(typ string) bool {
	for _, k := range storage.ValidTypes() {
		if strings.EqualFold(typ, k) {
			return false
		}
	}

	return true
}
