# Steps

1. download script from <https://gist.github.com/dave-tucker/5b1c5be9f9413a60d96b0758d889fa01>
2. Run `sudo perf record -k 1 -e cpu-clock -a  ./ebpf-proc-hybrid` for 60 seconds, this should generate file `perf.data`
3. Run `sudo perf script -s ebpf-overhead.py`

```bash
❯ sudo perf script -s ebpf-overhead.py
comm: ThreadPoolForeg, bpf_prog_e8932b6bae2b9745_restrict_filesystems: 0.01%,
comm: sh, bpf_prog_e8932b6bae2b9745_restrict_filesystems: 0.10%,
comm: SchedulerRunner, bpf_prog_e8932b6bae2b9745_restrict_filesystems: 0.03%,



bpf_prog_e8932b6bae2b9745_restrict_filesystems
        min: 0.01%
        max: 0.10%
        median: 0.03%
        mean: 0.05%
        stdDev: 0.04%
```
