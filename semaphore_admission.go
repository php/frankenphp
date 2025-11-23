package frankenphp

import (
	"context"

	"golang.org/x/sync/semaphore"
)

func acquireSemaphoreWithAdmissionControl(
	sem *semaphore.Weighted,
	scaleChan chan *frankenPHPContext,
	fc *frankenPHPContext,
) error {
	if maxWaitTime > 0 && scaleChan != nil {
		ctx, cancel := context.WithTimeout(context.Background(), minStallTime)
		err := sem.Acquire(ctx, 1)
		cancel()

		if err != nil {
			select {
			case scaleChan <- fc:
			default:
			}

			ctx, cancel := context.WithTimeout(context.Background(), maxWaitTime)
			defer cancel()

			if err := sem.Acquire(ctx, 1); err != nil {
				return ErrMaxWaitTimeExceeded
			}
		}
	} else if maxWaitTime > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), maxWaitTime)
		defer cancel()

		if err := sem.Acquire(ctx, 1); err != nil {
			return ErrMaxWaitTimeExceeded
		}
	} else if scaleChan != nil {
		ctx, cancel := context.WithTimeout(context.Background(), minStallTime)
		err := sem.Acquire(ctx, 1)
		cancel()

		if err != nil {
			select {
			case scaleChan <- fc:
			default:
			}

			if err := sem.Acquire(context.Background(), 1); err != nil {
				return ErrMaxWaitTimeExceeded
			}
		}
	} else {
		if err := sem.Acquire(context.Background(), 1); err != nil {
			return ErrMaxWaitTimeExceeded
		}
	}

	return nil
}
