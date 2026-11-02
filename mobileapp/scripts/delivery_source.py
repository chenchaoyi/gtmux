"""Exact-source CI gate shared by platform delivery workflows."""
import re


class DeliveryError(Exception):
    pass


def validate_ci(commit, ci_runs):
    if not re.fullmatch(r"[0-9a-f]{40}", commit):
        raise DeliveryError("Use an exact commit ID.")
    runs = [r for r in ci_runs if r.get("head_sha") == commit and r.get("head_branch") == "main" and r.get("event") == "push"]
    latest = max(runs, key=lambda r: (r.get("run_number", 0), r.get("run_attempt", 0)), default={})
    if latest.get("status") != "completed" or latest.get("conclusion") != "success":
        raise DeliveryError("Latest main CI for this exact commit must pass first.")
