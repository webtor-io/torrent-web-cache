package services

const (
	PRELOAD_QUEUE_CONCURRENCY = 3
)

type PreloadQueue struct {
	pp     *PreloadPiecePool
	ch     chan func()
	closed bool
	inited bool
}

func NewPreloadQueue(pp *PreloadPiecePool) *PreloadQueue {
	return &PreloadQueue{
		pp: pp,
		ch: make(chan func()),
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
	if !s.inited {
		s.inited = true
		for i := 0; i < PRELOAD_QUEUE_CONCURRENCY; i++ {
			go func() {
				for i := range s.ch {
					i()
				}
			}()
		}

	}
	s.ch <- func() {
		s.pp.Preload(src, h, p, q)
	}
}
