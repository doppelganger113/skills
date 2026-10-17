# Python

Contents
- Static checks
- Language pitfalls
- Async
- I/O, time and resources
- Databases, ORMs and workers
- Security

## Static checks

| Command | Class | Notes |
|---|---|---|
| `ruff check --no-fix .` | S if Ruff is installed outside the repository | Ruff loads no plugins; `--no-fix` overrides a `fix = true` in its config, which would rewrite files. |
| `pyright` | S if installed outside the repository through npm or as a standalone binary | The PyPI `pyright` wrapper downloads Node.js packages on first run, which makes it N. |
| `mypy .` | S only if the config names no `plugins` and mypy is installed outside the repository; otherwise E | Plugins import code; the Django plugin imports the project's settings. |
| `pytest`, `tox`, `nox`, `python -m ...` | E | `conftest.py`, `setup.py` and test code run. |
| `pip-audit` | E and N | Downloads the vulnerability database, and may install packages to resolve requirements. |
| `pip install`, `ruff check` without `--no-fix`, `black` | Never | They install or write files. |

## Language pitfalls

- Mutable default arguments (`def f(items=[])`) shared across calls.
- Closures created in a loop capture the variable, not its value (`lambda: i`).
- `except Exception: pass` or a bare `except:` that also catches `KeyboardInterrupt` and `SystemExit`, hiding real failures.
- `is` used to compare numbers or strings.
- Mutating a dict or set while iterating over it.
- Floats for money instead of `Decimal`.
- pandas and NumPy: `int64` overflow wraps silently; a missing value turns an integer column into floats; chained assignment that writes to a copy.

## Async

- Blocking calls inside `async def` (`requests`, `time.sleep`, synchronous database drivers, file I/O): they stall the event loop for every task.
- A coroutine called without `await`: it never runs.
- `asyncio.create_task(...)` without keeping a reference: the task can be garbage-collected before it finishes.
- `asyncio.gather` without `return_exceptions`: the first error propagates while the other tasks keep running.
- Cancellation: `except Exception` around an `await` that swallows `CancelledError` in older code, or cleanup that never runs on cancellation.

## I/O, time and resources

- `requests` has no default timeout: a hung server hangs the caller forever. The same goes for many other HTTP clients unless one is set.
- Files, sockets and connections opened without `with`.
- `datetime.now()` and `datetime.utcnow()` return naive datetimes; mixing naive and aware values raises, or compares wrongly across time zones.
- `open()` without `encoding=` uses the platform's default encoding before Python 3.15.
- Reading a whole upload or response into memory with no size cap.

## Databases, ORMs and workers

- N+1 queries through lazy-loaded relationships (SQLAlchemy relationships, Django related managers) inside loops.
- Read-modify-write in application code (`obj.count += 1; obj.save()`) instead of an atomic update (`F()` expressions, `UPDATE ... SET x = x + 1`) or a row lock.
- Django `select_for_update()` outside a transaction, and work that must be atomic without `transaction.atomic`.
- A SQLAlchemy `Session` shared across threads or requests.
- Celery and similar queues deliver at least once, especially with `acks_late`: tasks that aren't idempotent run twice; retries without a cap; no time limit.
- Prefork servers (gunicorn, Celery): connections or clients created at import time are shared across forked workers.

## Security

- `pickle`, `yaml.load` without a safe loader, `eval` or `exec` on untrusted data.
- `subprocess` with `shell=True` and interpolated input.
- SQL built with f-strings or `%` formatting instead of parameters.
- `os.path.join(base, user_path)`: an absolute `user_path` discards `base`.
- Debug modes or permissive CORS left on by a default setting.
