package analytics

import (
	"context"

	"github.com/ClickHouse/clickhouse-go/v2"
)

func CreateAnalyticsTable(ctx context.Context, conn clickhouse.Conn) error {
	queryDdl := `
		CREATE TABLE IF NOT EXISTS trip_events (
			trip_id          				String,
			region           				String,
			car_package             String,
			trip_status             Enum('searching', 'aborted', 'matched', 'active', 'completed', 'cancelled'),
			predicted_duration_mins Decimal(10, 2),
			actual_duration_mins    Decimal(10, 2),
			distance_km             Decimal(10, 2),
			pickup_lat              Float64,
			pickup_lng              Float64,
			rating                  UInt64,
			transaction_ref         String,
			driver_id               String,
			payment_provider        Enum('paystack', 'flutterwave', 'none'),
			payment_status   				Enum('pending', 'success', 'failed', 'aborted'),
			amount           				Decimal(10, 2),
			platform_fee     				Decimal(10, 2),
			driver_split     				Decimal(10, 2),
			driver_tip       				Decimal(10, 2),
			timestamp        				DateTime
		) ENGINE = MergeTree()
		PARTITION BY toYYYYMM(timestamp)
		ORDER BY (trip_id, region, transaction_ref, car_package, trip_status, payment_status, timestamp)
		TTL timestamp + INTERVAL 1 YEAR
	`

	if err := conn.Exec(ctx, queryDdl); err != nil {
		return err
	}

	return nil
}
