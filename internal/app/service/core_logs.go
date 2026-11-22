package service

// ReadLogs never waits for a core operation; startup failures remain readable.
func (s *CoreService) readLogs(offset uint64) (output string, start uint64, running bool) {
	running = s.logging.Load()
	output, start = s.logs.ReadSince(offset)
	return output, start, running
}

func (s *CoreService) usesProcessLogs() bool { return s.processLogs.Load() }
