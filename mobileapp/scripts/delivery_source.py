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


def validate_android_jobs(commit, jobs):
    """Accept the complete legacy gate or all three new gates; never a partial mix."""
    parallel = {"Android QA build and native policies", "Android phone emulator flows", "Android tablet emulator flows"}
    legacy = {"Android build and emulator flows"}
    if not isinstance(jobs, list) or not all(isinstance(job, dict) for job in jobs):
        raise DeliveryError("Android CI jobs are missing")
    relevant = [job for job in jobs if job.get("name") in parallel | legacy]
    names = {job.get("name") for job in relevant}
    required = parallel if names & parallel else legacy
    if names != required or len(relevant) != len(required) or any(
            job.get("head_sha") != commit or job.get("status") != "completed" or job.get("conclusion") != "success"
            for job in relevant):
        raise DeliveryError("Complete Android build, phone and tablet checks must pass for this exact source")
