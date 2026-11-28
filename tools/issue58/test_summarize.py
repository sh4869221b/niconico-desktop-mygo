import unittest

from summarize import segment_ends, summarize


class TrialSummary(unittest.TestCase):
    def test_distinguishes_requested_target_from_prefetched_other_segments(self):
        segments = segment_ends(("#EXTM3U", "#EXTINF:2.000,", "seg000.ts",
                                 "#EXTINF:2.250,", "seg001.ts", "#EXT-X-ENDLIST"))
        player = [
            dict(event="trial-start", trial=1, utc_ns=20, paused=False, ready=True),
            dict(event="seek", utc_ns=21, target_ns=2_000_000_000),
            dict(event="trial-end", trial=1, utc_ns=30),
        ]
        earlier = dict(event="request", utc_ns=11, method="GET", path="/seg000.ts", range="", seq=1)
        within = dict(event="request", utc_ns=22, method="GET", path="/seg001.ts", range="", seq=2)
        result, = summarize(player, [earlier, within], segments)
        self.assertEqual(result["target_segment"], "/seg001.ts")
        self.assertEqual(result["target_gets_before_trial"], 0)
        self.assertEqual(result["target_gets_in_trial"], 1)
        prefetched, = summarize(player, [earlier, within, dict(within, utc_ns=19, seq=3)], segments)
        self.assertEqual(prefetched["target_gets_before_trial"], 1)

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
