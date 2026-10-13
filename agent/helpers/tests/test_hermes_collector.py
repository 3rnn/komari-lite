"""Offline fixtures only: never opens the operator's Hermes home."""
import contextlib
import importlib.util
import json
import pathlib
import sqlite3
import tempfile
import unittest

MODULE = pathlib.Path(__file__).resolve().parents[1] / 'hermes_collector.py'

class CollectorTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = pathlib.Path(self.temp.name)
        self.db = self.root / 'hermes.db'
        with contextlib.closing(sqlite3.connect(self.db)) as c, c:
            c.executescript('''CREATE TABLE sessions(id TEXT PRIMARY KEY, rewind_count INTEGER, secret_content TEXT);
CREATE TABLE session_model_usage(session_id TEXT, model TEXT, billing_provider TEXT, billing_base_url TEXT,
billing_mode TEXT, task TEXT, api_call_count INTEGER, input_tokens INTEGER, output_tokens INTEGER,
cache_read_tokens INTEGER, cache_write_tokens INTEGER, reasoning_tokens INTEGER,
estimated_cost_usd REAL, actual_cost_usd REAL, cost_status TEXT, cost_source TEXT, first_seen REAL, last_seen REAL);
INSERT INTO sessions VALUES ('SECRET_SESSION',0,'DO NOT READ');
INSERT INTO session_model_usage VALUES ('SECRET_SESSION','test-model','openai','https://private.invalid','api','',1,100,20,30,4,5,0.01,0,'estimated','local',1,99);''')
        if MODULE.exists():
            spec = importlib.util.spec_from_file_location('collector', MODULE)
            assert spec is not None and spec.loader is not None
            self.m = importlib.util.module_from_spec(spec)
            spec.loader.exec_module(self.m)
    def tearDown(self):
        self.temp.cleanup()
    def collector(self):
        self.assertTrue(MODULE.exists(), 'restricted collector has not been implemented')
        return self.m.Collector(self.db, self.root / 'private', self.root / 'spool', interval=10)
    def enable(self):
        (self.root / 'spool').mkdir(exist_ok=True)
        (self.root / 'spool' / 'control.json').write_text('{"enabled":true}')
    def update(self, sql):
        with contextlib.closing(sqlite3.connect(self.db)) as c, c:
            c.executescript(sql)
    def test_disabled_never_reads_source_or_quota(self):
        h = self.collector()
        def forbidden(*args):
            self.fail('disabled collector touched Hermes')
        h.read_metrics = forbidden
        h.read_quota = forbidden
        self.assertEqual(h.tick(100)['status'], 'disabled')
        self.enable()
        (self.root/'spool'/'control.json').write_text('{"enabled":true,"database":"evil"}')
        self.assertEqual(h.tick(110)['status'], 'disabled')

    def test_recreated_row_with_higher_counters_rebaselines(self):
        h=self.collector(); self.enable(); h.tick(100)
        self.update('UPDATE session_model_usage SET first_seen=105,last_seen=105,input_tokens=900;')
        reset=h.tick(110)['samples'][0]
        self.assertIs(reset['coverage'],False);self.assertEqual(reset['input_tokens'],0)
        self.update('UPDATE session_model_usage SET input_tokens=910,last_seen=120;')
        self.assertEqual(h.tick(120)['samples'][-1]['input_tokens'], 10)

    def test_rewind_reset_and_proven_new_rows(self):
        h = self.collector(); self.enable(); h.tick(100)
        self.update('UPDATE sessions SET rewind_count=1; UPDATE session_model_usage SET input_tokens=150,last_seen=105;')
        reset=h.tick(105)['samples'][0]
        self.assertIs(reset['coverage'],False);self.assertEqual(reset['input_tokens'],0)
        self.update('UPDATE session_model_usage SET input_tokens=170,last_seen=110;')
        self.assertEqual(h.tick(110)['samples'][-1]['input_tokens'],20)
        self.update("INSERT INTO sessions VALUES ('NEW',0,'PRIVATE'); INSERT INTO session_model_usage SELECT 'NEW',model,billing_provider,billing_base_url,billing_mode,'vision',1,15,3,2,0,1,0,0,NULL,NULL,115,115 FROM session_model_usage WHERE session_id='SECRET_SESSION';")
        self.assertEqual(next(s for s in h.tick(120)['samples'] if s['task']=='vision')['input_tokens'],15)
        self.update('UPDATE session_model_usage SET input_tokens=0;')
        before=len(h.state['outbox']);report=h.tick(130)
        self.assertEqual(len(report['samples']),before+2)
        self.assertTrue(all(s['coverage'] is False for s in report['samples'][-2:]))

    def test_ack_ttl_boundaries_and_safe_gap(self):
        h = self.collector(); self.enable(); h.tick(100)
        self.update('UPDATE session_model_usage SET input_tokens=150,last_seen=110;')
        s = h.tick(110)['samples'][0]
        ack = self.root/'spool'/'acks'; ack.mkdir(exist_ok=True)
        (ack/s['sample_id']).write_text('')
        (ack/'not-a-uuid').write_text('')
        self.assertNotIn(s['sample_id'],[row['sample_id'] for row in h.tick(120)['samples']])
        self.update('UPDATE session_model_usage SET input_tokens=200,last_seen=1000;')
        before=h.state['outbox'][:]
        self.assertEqual(h.tick(1000)['samples'], before, 'long gap must rebaseline')
        self.update('UPDATE session_model_usage SET input_tokens=210,last_seen=1017;')
        row=h.tick(1017)['samples'][-1]
        self.assertEqual(row['ended_at']-row['started_at'],17)
        # Baselines expire after the long gap, but unacknowledged immutable
        # samples remain available for delivery until the relay writes ACKs.
        self.assertEqual(h.tick(1017+31*86400)['samples'], h.state['outbox'][:256])

    def test_exclusive_lock_and_read_only_queries(self):
        h = self.collector(); self.enable()
        with contextlib.closing(sqlite3.connect(self.db)) as c, c:
            c.execute('BEGIN EXCLUSIVE')
            self.assertEqual(h.tick(100)['status'],'database_unavailable')
        self.assertEqual(h.tick(110)['status'],'no_usage')
        self.assertNotIn('SELECT *',self.m.METRIC_SQL)
        self.assertNotIn('secret_content',self.m.METRIC_SQL)
        self.assertNotIn('immutable',self.m.METRIC_SQL)

    def test_quota_read_only_allowlisted_get_interval_and_account_identity(self):
        h = self.collector(); self.enable()
        self.assertTrue(hasattr(h, 'read_quota'), 'quota reader missing')
        import base64
        claims={'exp':5000,'https://api.openai.com/auth':{'chatgpt_account_id':'account-fixture'}}
        token='x.'+base64.urlsafe_b64encode(json.dumps(claims).encode()).decode().rstrip('=')+'.x'
        auth=self.root/'auth.json'
        auth.write_text(json.dumps({'providers':{'openai-codex':{'tokens':{'access_token':token,'refresh_token':'NEVER_USE'}}}}))
        h.auth_path=auth
        seen=[]
        def get(request):
            self.assertTrue(h.state_path.exists(),'quota attempt throttle must persist before network')
            self.assertEqual(json.loads(h.state_path.read_text())['quota_attempt_at'],100 if not seen else 1000)
            seen.append(request)
            self.assertEqual(request.full_url,'https://chatgpt.com/backend-api/wham/usage')
            self.assertEqual(request.get_method(),'GET')
            return {'plan_type':'plus','rate_limit':{'primary_window':{'limit_window_seconds':604800,'used_percent':25,'reset_at':2000},'secondary_window':{'limit_window_seconds':18000,'used_percent':None}},'secret':'RAW_NEVER_SAVE'}
        h.quota_get=get
        before=auth.read_bytes()
        q=h.tick(100)['quota']
        self.assertEqual(q['windows'][0]['label'],'Weekly')
        self.assertEqual(q['windows'][1]['used_percent'],None)
        self.assertEqual(q['status'],'available')
        self.assertEqual(len(q['account_key']),64)
        self.assertNotIn(token,json.dumps(h.state)); self.assertNotIn('RAW_NEVER_SAVE',json.dumps(h.state))
        h.tick(999); self.assertEqual(len(seen),1)
        h.tick(1000); self.assertEqual(len(seen),2)
        self.assertEqual(auth.read_bytes(),before)
        (self.root/'spool'/'control.json').write_text('{"enabled":false}')
        h.tick(3000); self.assertEqual(len(seen),2)

    def test_pool_skips_rate_limited_primary_and_reads_ready_fallback_quota(self):
        h=self.collector(); self.enable()
        import base64
        def token(account):
            claims={'exp':5000,'https://api.openai.com/auth':{'chatgpt_account_id':account}}
            return 'x.'+base64.urlsafe_b64encode(json.dumps(claims).encode()).decode().rstrip('=')+'.x'
        primary, backup = token('team-fixture'), token('plus-fixture')
        h.auth_path=self.root/'auth.json'
        h.auth_path.write_text(json.dumps({'credential_pool':{'openai-codex':[{'access_token':primary,'priority':0,'last_status':'rate-limited','last_error_reset_at':9999999999},{'access_token':backup,'priority':1,'last_status':'ready'}]}}))
        seen=[]
        def get(request):
            seen.append(request)
            self.assertEqual(request.get_header('Authorization'),'Bearer '+backup)
            return {'plan_type':'plus','rate_limit':{'primary_window':{'limit_window_seconds':18000,'used_percent':20}}}
        h.quota_get=get
        quota=h.tick(100)['quota']
        self.assertEqual(len(seen),1)
        self.assertEqual(quota['status'],'available')
        self.assertEqual(quota['plan'],'plus')
        self.assertEqual(len(quota['account_key']),64)
        self.assertNotIn(primary,json.dumps(h.state))
        self.assertNotIn(backup,json.dumps(h.state))

    def test_pool_skips_primary_marked_rate_limited_by_failure_reason(self):
        h=self.collector(); self.enable()
        import base64
        def token(account):
            claims={'exp':5000,'https://api.openai.com/auth':{'chatgpt_account_id':account}}
            return 'x.'+base64.urlsafe_b64encode(json.dumps(claims).encode()).decode().rstrip('=')+'.x'
        team, plus = token('team-fixture'), token('plus-fixture')
        h.auth_path=self.root/'auth.json'
        h.auth_path.write_text(json.dumps({'credential_pool':{'openai-codex':[{'access_token':team,'priority':0,'last_status':'ok','failure_reason':'rate_limit'},{'access_token':plus,'priority':1,'last_status':'ready'}]}}))
        def get(request):
            self.assertEqual(request.get_header('Authorization'),'Bearer '+plus)
            return {'plan_type':'plus','rate_limit':{'primary_window':{'limit_window_seconds':18000,'used_percent':20}}}
        h.quota_get=get
        quota=h.tick(100)['quota']
        self.assertEqual(quota['plan'],'plus')
        self.assertNotIn(team,json.dumps(h.state))

    def test_empty_quota_response_is_unavailable_not_available(self):
        h=self.collector()
        import base64
        claims={'exp':5000,'https://api.openai.com/auth':{'chatgpt_account_id':'fixture'}}
        token='x.'+base64.urlsafe_b64encode(json.dumps(claims).encode()).decode().rstrip('=')+'.x'
        h.auth_path=self.root/'auth.json'
        h.auth_path.write_text(json.dumps({'providers':{'openai-codex':{'tokens':{'access_token':token}}}}))
        for payload in ({}, {'rate_limit':{'primary_window':{'used_percent':None}}}):
            h.quota_get=lambda request:payload
            quota=h.read_quota(100)
            self.assertEqual(quota['status'],'unavailable')
            self.assertNotIn('last_success_at',quota)

    def test_delivery_diagnostics_require_agent_capability_marker(self):
        h=self.collector();self.enable()
        (self.root/'spool'/'delivery.json').write_text(json.dumps({'last_attempt_at':1,'last_success_at':0,'consecutive_failures':1,'last_error_class':'http_5xx'}))
        legacy=h.tick(100)
        self.assertNotIn('delivery',legacy)
        self.assertNotIn('pending_sample_count',legacy)
        (self.root/'spool'/'protocol.json').write_text('{"version":2}')
        enriched=h.tick(110)
        self.assertEqual(enriched['delivery']['last_error_class'],'http_5xx')
        self.assertEqual(enriched['pending_sample_count'],len(h.state['outbox']))

    def test_quota_unidentifiable_expired_and_redirects_fail_closed(self):
        h=self.collector(); self.enable()
        self.assertTrue(hasattr(self.m,'NoRedirect'),'redirect guard missing')
        h.auth_path=self.root/'auth.json'
        h.auth_path.write_text('{"providers":{"openai-codex":{"tokens":{"access_token":"not-jwt"}}}}')
        h.quota_get=lambda request:self.fail('unidentifiable token was sent')
        self.assertEqual(h.tick(100)['quota']['status'],'unavailable')
        self.assertNotIn('account_key',h.tick(110)['quota'])
        with self.assertRaises(Exception):
            self.m.NoRedirect().redirect_request(None,None,302,'',{},'https://evil.invalid')

    def test_cli_disabled_without_source_and_process_lock(self):
        import subprocess, sys
        args=[sys.executable,str(MODULE),'--database',str(self.root/'absent.db'),'--state-dir',str(self.root/'private'),'--spool',str(self.root/'spool'),'--once']
        run=subprocess.run(args,capture_output=True,text=True)
        self.assertEqual(run.returncode,0,run.stderr)
        self.assertTrue((self.root/'spool'/'report.json').exists(),'CLI must produce a disabled report')
        self.assertEqual(json.loads((self.root/'spool'/'report.json').read_text())['status'],'disabled')
        import fcntl
        with (self.root/'private'/'collector.lock').open('a') as lock:
            fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
            run=subprocess.run(args,capture_output=True,text=True)
            self.assertNotEqual(run.returncode,0,'second helper must not run')
        (self.root/'spool'/'control.json').write_text('{"enabled":1}')
        h=self.collector()
        self.assertEqual(h.tick(100)['status'],'disabled','only JSON boolean allowed')

    def test_invalid_counters_cardinality_and_private_query_allowlist(self):
        h=self.collector();self.enable();h.tick(100)
        self.update('UPDATE session_model_usage SET input_tokens=1.5;')
        self.assertEqual(h.tick(110)['status'],'database_unavailable','fractional counters must not truncate')
        self.update('UPDATE session_model_usage SET input_tokens=100;')
        denied=[];connect=self.m.sqlite3.connect
        allowed={'session_model_usage':set('session_id model billing_provider billing_base_url billing_mode task api_call_count input_tokens output_tokens cache_read_tokens cache_write_tokens reasoning_tokens first_seen last_seen'.split()),'sessions':{'id','rewind_count'}}
        def safe_connect(*a,**kw):
            c=connect(*a,**kw)
            def guard(action,table,column,*_):
                if action==sqlite3.SQLITE_READ and column not in allowed.get(table,set()):
                    denied.append((table,column));return sqlite3.SQLITE_DENY
                return sqlite3.SQLITE_OK
            c.set_authorizer(guard);return c
        self.m.sqlite3.connect=safe_connect
        try:
            self.assertEqual(h.tick(120)['status'],'no_usage')
            self.assertEqual(denied,[])
        finally:self.m.sqlite3.connect=connect
        with contextlib.closing(connect(self.db)) as c,c:
            row=c.execute('SELECT * FROM session_model_usage').fetchone()
            c.executemany('INSERT INTO session_model_usage VALUES ('+','.join('?'*18)+')',[(f'row{i}',*row[1:]) for i in range(4097)])
        self.assertEqual(h.tick(130)['status'],'database_unavailable')
        self.assertLessEqual(len(h.state['baseline']),4096)
        # Seed only sanitized aggregate rows; stale ACKs remain bounded, but
        # unacknowledged sample IDs must remain durable for retry.
        sample={'sample_id':'11111111-1111-4111-8111-111111111111','ended_at':130}
        h.state['outbox']=[dict(sample,sample_id=str(self.m.uuid.uuid4())) for _ in range(4200)]
        h.tick(140);self.assertEqual(len(h.state['outbox']),4200,'unacknowledged samples must never be silently discarded')
        ack=self.root/'spool'/'acks'
        (ack/'22222222-2222-4222-8222-222222222222').write_text('')
        h.tick(150);self.assertFalse((ack/'22222222-2222-4222-8222-222222222222').exists(),'orphan ACKs must be bounded')

    def test_quota_failure_preserves_only_sanitized_stale_snapshot(self):
        h=self.collector();self.enable()
        h.state['quota']={'provider':'openai-codex','status':'available','account_key':'a'*64,'observed_at':100,'last_success_at':100,'windows':[{'label':'Weekly','used_percent':25}]}
        h.state['quota_attempt_at']=100
        h.quota_identity=lambda:('unused',None,5000,'a'*64)
        h.read_quota=lambda now:{'provider':'openai-codex','status':'unavailable','account_key':'a'*64,'observed_at':now,'windows':[]}
        q=h.tick(1000)['quota']
        self.assertEqual(q['status'],'stale');self.assertEqual(q['last_success_at'],100)
        self.assertEqual(q['windows'][0]['used_percent'],25)

    def test_account_switch_clears_cached_quota_and_forces_refresh(self):
        h=self.collector();self.enable()
        import base64
        def auth(account):
            claims={'exp':5000,'https://api.openai.com/auth':{'chatgpt_account_id':account}}
            token='x.'+base64.urlsafe_b64encode(json.dumps(claims).encode()).decode().rstrip('=')+'.x'
            path=self.root/'auth.json'
            path.write_text(json.dumps({'providers':{'openai-codex':{'tokens':{'access_token':token}}}}))
            return path
        h.auth_path=auth('old-account')
        calls=[]
        h.quota_get=lambda request:(calls.append(request) or {'plan_type':'plus','rate_limit':{'primary_window':{'limit_window_seconds':18000,'used_percent':10}}})
        old=h.tick(100)['quota']
        h.auth_path=auth('new-account')
        h.quota_get=lambda request:(calls.append(request) or {'plan_type':'team','rate_limit':{'primary_window':{'limit_window_seconds':18000,'used_percent':20}}})
        report=h.tick(110)
        self.assertEqual(len(calls),2,'identity replacement must bypass the quota throttle')
        self.assertNotEqual(report['quota']['account_key'],old['account_key'])
        self.assertEqual(report['quota']['plan'],'team')

    def test_outbox_boundary_does_not_append_partial_interval(self):
        h=self.collector();self.enable();h.tick(100)
        self.update("INSERT INTO sessions VALUES ('SECOND',0,'PRIVATE'); INSERT INTO session_model_usage SELECT 'SECOND',model,billing_provider,billing_base_url,billing_mode,'second',api_call_count,input_tokens,output_tokens,cache_read_tokens,cache_write_tokens,reasoning_tokens,estimated_cost_usd,actual_cost_usd,cost_status,cost_source,101,101 FROM session_model_usage;")
        h.tick(101)
        h.state['outbox']=[{'sample_id':str(self.m.uuid.uuid4()),'ended_at':100} for _ in range(4095)]
        baseline=json.loads(json.dumps(h.state['baseline']))
        self.update('UPDATE session_model_usage SET input_tokens=110,last_seen=110;')
        report=h.tick(110)
        self.assertEqual(report['status'],'stale')
        self.assertEqual(len(h.state['outbox']),4095)
        self.assertEqual(h.state['baseline'],baseline)
        self.assertEqual(h.state['last'],101)

    def test_partial_group_reset_never_claims_complete_rate_coverage(self):
        h=self.collector();self.enable()
        self.update("INSERT INTO sessions VALUES ('SECOND',0,'PRIVATE'); INSERT INTO session_model_usage SELECT 'SECOND',model,billing_provider,billing_base_url,billing_mode,task,api_call_count,input_tokens,output_tokens,cache_read_tokens,cache_write_tokens,reasoning_tokens,estimated_cost_usd,actual_cost_usd,cost_status,cost_source,first_seen,last_seen FROM session_model_usage;")
        h.tick(100)
        self.update("UPDATE session_model_usage SET input_tokens=0 WHERE session_id='SECRET_SESSION'; UPDATE session_model_usage SET input_tokens=105 WHERE session_id='SECOND';")
        s=h.tick(110)['samples'][0]
        self.assertEqual(s['input_tokens'],5)
        self.assertIs(s['coverage'],False,'partial reset makes sampled rates unknown')
        self.assertEqual(s['reset_count'],1)

    def test_all_group_reset_emits_explicit_uncovered_zero_sample(self):
        h=self.collector();self.enable();h.tick(100)
        self.update('UPDATE session_model_usage SET input_tokens=0;')
        report=h.tick(110)
        self.assertEqual(report['status'],'collecting')
        self.assertEqual(len(report['samples']),1)
        sample=report['samples'][0]
        self.assertIs(sample['coverage'],False)
        self.assertGreater(sample['reset_count'],0)
        self.assertEqual(sum(sample[k] for k in self.m.COUNTERS),0)

    def test_disabled_retention_expires_private_baselines_and_quota_without_reads(self):
        h=self.collector();self.enable();h.tick(100)
        h.state['quota']={'status':'available','observed_at':100,'last_success_at':100}
        (self.root/'spool'/'control.json').write_text('{"enabled":false}')
        h.read_metrics=lambda:self.fail('disabled source read')
        h.tick(100+31*86400)
        self.assertEqual(h.state['baseline'],{})
        self.assertNotIn('quota',h.state)

    def test_zero_usage_samples_prove_elapsed_coverage_without_generation_claims(self):
        h=self.collector();self.enable();h.tick(100)
        report=h.tick(117)
        self.assertEqual(report['status'],'no_usage')
        self.assertTrue(report['samples'],'idle covered intervals need explicit zero samples')
        s=report['samples'][0]
        self.assertIs(s['coverage'],True)
        self.assertEqual((s['started_at'],s['ended_at']),(100,117))
        self.assertEqual(sum(s[k] for k in self.m.COUNTERS),0)

    def test_initial_baseline_then_canonical_delta_durable_retry(self):
        h = self.collector()
        self.enable()
        self.assertEqual(h.tick(100)['samples'], [])
        self.update('UPDATE session_model_usage SET api_call_count=3,input_tokens=150,output_tokens=40,cache_read_tokens=60,cache_write_tokens=8,reasoning_tokens=10,last_seen=110;')
        report = h.tick(110)
        s = report['samples'][0]
        self.assertEqual([s[k] for k in ('calls','input_tokens','output_tokens','cache_read_tokens','cache_write_tokens','reasoning_tokens')], [2,50,20,30,4,5])
        self.assertEqual((s['started_at'],s['ended_at']), (100,110))
        self.assertNotIn('SECRET_SESSION', json.dumps(report))
        self.assertNotIn('private.invalid', json.dumps(report))
        self.assertEqual(self.collector().tick(120)['samples'][0], s)

if __name__ == '__main__':
    unittest.main()
