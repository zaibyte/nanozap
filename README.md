nanoZap
===

nanoZap is a nanosecond scale logging system for Go based on uber-go/zap.

zap provides stable & robust foundation, it is easy to drop modules that we don't need,
and the remaining parts are still working well.

## Modifications

### Improvement

Avg. latency is 30ns/op. (6x faster). No stall anymore, someone has implemented a buffer for log, but it's not enough,
because when disk flushes the buffer it may cause disk stall and your program will be blocked for a long time.
In this version, nothing is blocking, and I'll drop messages if there are too many 
(e.g., if there are 1millions/seconds, nanoZap may drop some).

1. remove console encoder
2. wait-free log write
3. async disk flush
4. add an internal log rolling package

### Shrink

I don't need these features in my project, so I removed them...

1. float64 replace by int64 in epochTimeEncoder
2. remove stack / no caller
3. remove std error output in Logger
4. remove Any Type
5. remove fields support, only message
6. no options (no logger clone too)
7. remove DPanicLevel
8. no logger name
9. no sample 
10. ...