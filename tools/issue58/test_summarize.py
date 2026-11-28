import unittest

from summarize import summarize


class TrialSummary(unittest.TestCase):
    def test_pending_position_and_paused_trial_are_not_counted_as_resume(self):
        player = [
            dict(event="trial-start", trial=1, utc_ns=0, paused=False, ready=True),
            dict(event="seek", utc_ns=1_000_000_000, target_ns=80_000_000_000),
            dict(event="sample", utc_ns=1_010_000_000, position_ns=80_000_000_000, playing=True),
            dict(event="sample", utc_ns=1_150_000_000, confirmed_position_ns=80_000_000_000, playing=True),
            dict(event="sample", utc_ns=1_410_000_000, confirmed_position_ns=80_260_000_000, playing=True),
            dict(event="trial-end", trial=1, utc_ns=2_000_000_000),
            dict(event="trial-start", trial=2, utc_ns=3_000_000_000, paused=True, ready=True),
            dict(event="seek", utc_ns=3_200_000_000, target_ns=20_000_000_000),
            dict(event="sample", utc_ns=3_400_000_000, confirmed_position_ns=20_000_000_000, playing=False),
            dict(event="trial-interrupted", trial=2, utc_ns=4_000_000_000),
        ]
        http = [dict(event="request", utc_ns=1_000_000_000 + seq, method="GET", path="/seg040.ts",
                     range="bytes=0-", seq=seq) for seq in (1, 2)]
        http.append(dict(event="response", request_id=1, outcome="client-disconnected"))
        played, paused = summarize(player, http)
        self.assertEqual(played["last_seek_to_confirmed_ms"], 150)
        self.assertEqual(played["last_seek_to_250ms_progress_ms"], 410)
        self.assertEqual(played["extra_gets_same_path_and_range"], 1)
        self.assertEqual(played["http_outcomes"]["client-disconnected"], 1)
        self.assertEqual(paused["last_seek_to_confirmed_ms"], 200)
        self.assertIsNone(paused["last_seek_to_250ms_progress_ms"])
        self.assertFalse(paused["closed_by_marker"])
