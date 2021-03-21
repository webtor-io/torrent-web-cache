package services

const (
	PRELOAD_QUEUE_CONCURRENCY = 3
)

type PreloadQueue struct {
	pp     *PreloadPiecePool
	ch     chan func()
	closed bool
}

func NewPreloadQueue(pp *PreloadPiecePool) *PreloadQueue {
	ch := make(chan func())
	for i := 0; i < PRELOAD_QUEUE_CONCURRENCY; i++ {
		go func() {
			for i := range ch {
				i()
			}
		}()
	}
	return &PreloadQueue{
		pp: pp,
		ch: ch,
	}
}

func (s *PreloadQueue) Close() {
	if s.closed {
		return
	}
	s.closed = true
	close(s.ch)
}

func (s *PreloadQueue) Push(src string, h string, p string, q string) {
	s.ch <- func() {
		s.pp.Preload(src, h, p, q)
	}
}
