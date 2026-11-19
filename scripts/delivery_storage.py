"""Keep project delivery writers out of the application's managed roots."""
from pathlib import Path


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
