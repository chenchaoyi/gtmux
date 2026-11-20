"""Keep project delivery writers out of the application's managed roots."""
from contextlib import contextmanager
import os
from pathlib import Path
import secrets
import stat


def at_or_under(path, root):
    for part in (path, *path.parents):
        if part == root:
            return True
        try:
            if part.samefile(root):
                return True
        except FileNotFoundError:
            pass
    return False


def require_external_path(path):
    proposed = path.absolute()
    try:
        actual = proposed.resolve()
    except RuntimeError as error:
        raise ValueError("Delivery output path could not be resolved") from error
    home = Path.home()
    for managed in (home / ".config/gtmux", home / ".local/share/gtmux"):
        if at_or_under(proposed, managed) or at_or_under(actual, managed):
            raise ValueError("Delivery output must stay outside gtmux configuration and data")
    return actual


def _external_child(root, relative):
    relative = Path(relative)
    if relative.is_absolute() or not relative.parts or ".." in relative.parts:
        raise ValueError("Delivery output must be a child of its external root")
    path = require_external_path(root) / relative
    require_external_path(path)
    return path


@contextmanager
def _directory_fd(path):
    # Walk the canonical external root and then its children without following
    # a symlink introduced between validation and opening an output.
    flags = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW
    fd = os.open(path.anchor, flags)
    try:
        for name in path.parts[1:]:
            try:
                child = os.open(name, flags, dir_fd=fd)
            except FileNotFoundError:
                try:
                    os.mkdir(name, mode=0o700, dir_fd=fd)
                except FileExistsError:
                    pass
                child = os.open(name, flags, dir_fd=fd)
            os.close(fd)
            fd = child
        yield fd
    finally:
        os.close(fd)


def prepare_delivery_directory(root, relative):
    path = _external_child(root, relative)
    with _directory_fd(path):
        return path


@contextmanager
def open_delivery_output(root, relative, mode, *, atomic=False):
    path = _external_child(root, relative)
    with _directory_fd(path.parent) as directory:
        try:
            existing = os.stat(path.name, dir_fd=directory, follow_symlinks=False)
        except FileNotFoundError:
            existing = None
        if existing is not None and not stat.S_ISREG(existing.st_mode):
            raise ValueError("Delivery output must be a regular file")

        name = ".delivery-" + secrets.token_hex(12) if atomic else path.name
        flags = os.O_WRONLY | os.O_CREAT | os.O_NOFOLLOW | os.O_NONBLOCK
        if atomic:
            flags |= os.O_EXCL
        elif mode == "a":
            flags |= os.O_APPEND
        fd = os.open(name, flags, 0o600, dir_fd=directory)
        try:
            with os.fdopen(fd, mode) as output:
                info = os.fstat(output.fileno())
                if not stat.S_ISREG(info.st_mode) or info.st_nlink != 1:
                    raise ValueError("Delivery output must be an unshared regular file")
                if not atomic and mode != "a":
                    os.ftruncate(output.fileno(), 0)
                yield output
            if atomic:
                os.replace(name, path.name, src_dir_fd=directory, dst_dir_fd=directory)
        finally:
            if atomic:
                try:
                    os.unlink(name, dir_fd=directory)
                except FileNotFoundError:
                    pass
