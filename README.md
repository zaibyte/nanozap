# nanozap
Nanozap is a nanosecond scale logging system for Go based on uber-go/zap.

1. no console encoder

2. use nanaoseconds as time

3. float64 replace by int64 in epochTimeEncoder

4. no stack

5. no caller

6. no error output in Logger

7. no Any Type