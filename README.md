# nanozap
Nanozap is a nanosecond scale logging system for Go based on uber-go/zap.

zap provides stable & robust foundation, it easy to drop modules that we don't need,
and the remaining parts are still working well.

It's designed for Zai with these changes:

1. no console encoder

2. use nanaoseconds as time

3. float64 replace by int64 in epochTimeEncoder

4. no stack

5. no caller

6. no error output in Logger

7. no Any Type

8. remove fields support, only message

9. no options (no logger clone too)

10. remove DPanicLevel

11. no logger name

12. async logger write

13. no sampler

14. add an internal log rolling package

Avg. latency is 30ns/op. (6x faster)

## WIP

1. refine codes