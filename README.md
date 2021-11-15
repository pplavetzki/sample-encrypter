# pprof help

* [gorilla mux leak - graphically show?](https://stackoverflow.com/questions/59465996/go-net-http-leaks-memory-in-high-load)
* [memory usage](https://linuxhint.com/check_memory_usage_process_linux/)
* [json generator](https://www.json-generator.com/#)
* [memory leak](https://yuriktech.com/2020/11/07/Golang-Memory-Leaks/)
* [pprof](https://jvns.ca/blog/2017/09/24/profiling-go-with-pprof/)
* [pprof](https://www.freecodecamp.org/news/how-i-investigated-memory-leaks-in-go-using-pprof-on-a-large-codebase-4bec4325e192/)

```bash
hey -m POST -D data/message.json -c 2 -t 0 -n 20 -H 'content-type: application/json' http://127.0.0.1:9090/encrypt
```

```bash
expvarmon -ports=6060 -i 500ms
```