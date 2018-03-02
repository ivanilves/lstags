package wait

// Until iterates over buffered error channel and:
// * upon receiving non-nil value from the channel, makes an early return with this value
// * if no non-nil values were received from iteration over the channel, it just returns nil
// Used to wait for a series of goroutines, launched alltogether from the same loop, to finish.
func Until(done chan error) error {
	i := 0

	for err := range done {
		if err != nil {
			return err
		}

		i++

		if i >= cap(done) {
			close(done)
		}
	}

	return nil
}
