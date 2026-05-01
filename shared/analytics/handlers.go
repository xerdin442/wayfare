package analytics

import (
	"context"

	"github.com/xerdin442/wayfare/shared/messaging"
)

func HandleTripLifecycleMetrics(ctx context.Context, p messaging.AmqpDeliveryPayload) error

func HandlePaymentMetrics(ctx context.Context, p messaging.AmqpDeliveryPayload) error
