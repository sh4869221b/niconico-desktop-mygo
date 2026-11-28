"""Summarize marked local HLS trials using Windows player and server JSONL."""
import argparse
from collections import Counter
from decimal import Decimal
import json
from pathlib import Path


def read_rows(path):
    with path.open(encoding="utf-8-sig") as log:
        return [json.loads(line) for line in log if line.strip()]


def ms(ns):
    return round(ns / 1_000_000, 2)


def playback_timing(seeks, samples, paused):
    if not seeks:
        return None, None
    last = seeks[-1]
    confirmed_ns = None
    movement_start = None
    for row in samples:
        if row["utc_ns"] < last["utc_ns"]:
            continue
        position = row.get("confirmed_position_ns")
        if position is None or row.get("buffering") or row.get("failure"):
            movement_start = None
            continue
        near_target = abs(position - last["target_ns"]) <= 2_000_000_000
        if confirmed_ns is None and near_target:
            confirmed_ns = ms(row["utc_ns"] - last["utc_ns"])
        if paused or not row.get("playing"):
            movement_start = None
            continue
        if movement_start is not None and position < movement_start:
            movement_start = None
        if movement_start is None and near_target:
            movement_start = position
        if movement_start is not None and position - movement_start >= 250_000_000:
            return confirmed_ns, ms(row["utc_ns"] - last["utc_ns"])
    return confirmed_ns, None


def segment_ends(lines):
    end = 0
    duration = None
    segments = []
    for line in map(str.strip, lines):
        if line.startswith("#EXTINF:"):
            duration = int(Decimal(line.removeprefix("#EXTINF:").split(",", 1)[0]) * 1_000_000_000)
        elif duration is not None and line and not line.startswith("#"):
            end += duration
            segments.append((end, "/" + line.lstrip("/")))
            duration = None
    return segments


def summarize(player, http, segments=()):
    starts = {row["trial"]: row for row in player if row["event"] == "trial-start"}
    ends = {row["trial"]: row for row in player if row["event"] in ("trial-end", "trial-interrupted")}
    responses = {row["request_id"]: row for row in http if row["event"] == "response"}
    result = []
    segment_gets = [row for row in http if row["event"] == "request"
                    and row["method"] == "GET" and row["path"].endswith(".ts")]
    for number, start in sorted(starts.items()):
        end = ends.get(number)
        begin = start["utc_ns"]
        stop = end["utc_ns"] if end else max(row["utc_ns"] for row in player)
        events = [row for row in player if begin <= row["utc_ns"] <= stop]
        samples = [row for row in events if row["event"] == "sample"]
        seeks = [row for row in events if row["event"] == "seek"]
        requests = [row for row in segment_gets if begin <= row["utc_ns"] <= stop]
        keys = Counter((row["path"], row["range"]) for row in requests)
        outcomes = Counter(responses.get(row["seq"], {}).get("outcome", "response-not-recorded")
                           for row in requests)
        confirmed, advanced = playback_timing(seeks, samples, start["paused"])
        trial = {
            "trial": number, "closed_by_marker": end is not None and end["event"] == "trial-end",
            "ready_at_start": start["ready"], "paused_at_start": start["paused"],
            "seek_calls": len(seeks), "segment_gets": len(requests),
            "extra_gets_same_path_and_range": sum(count - 1 for count in keys.values()),
            "http_outcomes": outcomes, "last_seek_to_confirmed_ms": confirmed,
            "last_seek_to_250ms_progress_ms": advanced,
            "failures": sorted({row["failure"] for row in samples if row.get("failure")}),
        }
        if segments and seeks:
            target = seeks[-1]["target_ns"]
            path = next((path for end, path in segments if target < end), None)
            if path is not None:
                trial.update(
                    target_segment=path,
                    target_gets_before_trial=sum(row["path"] == path and row["utc_ns"] < begin for row in segment_gets),
                    target_gets_in_trial=sum(row["path"] == path for row in requests),
                )
        result.append(trial)
    return result


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("player", type=Path)
    parser.add_argument("http", type=Path)
    parser.add_argument("--playlist", type=Path, help="local media playlist for the final seek target's segment")
    args = parser.parse_args()
    http = read_rows(args.http)
    player = read_rows(args.player)
    segments = segment_ends(args.playlist.read_text(encoding="utf-8-sig").splitlines()) if args.playlist else ()
    print(json.dumps({"http_config": next((r for r in http if r["event"] == "config"), None),
                      "startup": next((r for r in player if r["event"] == "startup-result"), None),
                      "native_failure": next((r for r in player if r["event"] == "native-stream-failure"), None),
                      "trials": summarize(player, http, segments)}, ensure_ascii=False, indent=2))
