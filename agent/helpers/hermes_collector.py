#!/usr/bin/env python3
"""Restricted, metric-only Hermes observer. Never import Hermes runtime/auth code.

Private state is distinct from the aggregate-only Agent spool. Source mounts must
be read-only; live WAL/SHM failures are unavailable, never immutable fallbacks.
"""
import argparse
import base64
import contextlib
import datetime
import math
import itertools
import time
import urllib.request
import urllib.error
import hashlib
import json
import os
import pathlib
import re
import sqlite3
import uuid

COUNTERS = ('calls', 'input_tokens', 'output_tokens', 'cache_read_tokens',
            'cache_write_tokens', 'reasoning_tokens')
MAX_OUTBOX = 4096
# No SELECT *, message, prompt, conversation or session content columns.
METRIC_SQL = '''SELECT u.session_id,u.model,u.billing_provider,u.billing_base_url,
u.billing_mode,u.task,u.api_call_count,u.input_tokens,u.output_tokens,
u.cache_read_tokens,u.cache_write_tokens,u.reasoning_tokens,u.first_seen,u.last_seen
FROM session_model_usage u ORDER BY u.last_seen DESC LIMIT 4097'''


def atomic_json(path, value, mode=0o600):
    path = pathlib.Path(path)
    temp = path.with_name('.' + path.name + '.' + str(uuid.uuid4()))
    fd = os.open(temp, os.O_WRONLY | os.O_CREAT | os.O_EXCL, mode)
    try:
        with os.fdopen(fd, 'w') as out:
            json.dump(value, out, allow_nan=False, separators=(',', ':'))
            out.flush()
            os.fsync(out.fileno())
        os.replace(temp, path)
        d = os.open(path.parent, os.O_RDONLY)
        try:
            os.fsync(d)
        finally:
            os.close(d)
    finally:
        temp.unlink(missing_ok=True)


def identifier(value, default='unknown'):
    value = value or default
    if not isinstance(value, str) or not re.fullmatch(r'[A-Za-z0-9_.:/-]{1,128}', value):
        return default
    return value


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        raise urllib.error.URLError('quota redirects are forbidden')


def quota_get(request):
    # Ignore proxy environment: the token is only sent to the official endpoint.
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    with opener.open(request, timeout=5) as response:
        data = response.read(65537)
    if len(data)>65536:
        raise ValueError('quota response too large')
    return json.loads(data)


def bounded_json(path, limit):
    with pathlib.Path(path).open('rb') as source:
        data = source.read(limit+1)
    if len(data)>limit:
        raise ValueError('local JSON too large')
    return json.loads(data)


class Collector:
    def __init__(self, database, state_dir, spool, interval=10, auth_path=None, quota_interval=900, retention_days=30, account_alias=None):
        self.database = pathlib.Path(database)
        self.private = pathlib.Path(state_dir)
        self.spool = pathlib.Path(spool)
        self.interval = max(1,min(60,float(interval)))
        self.auth_path = pathlib.Path(auth_path) if auth_path else None
        self.quota_interval = max(900,float(quota_interval))
        self.retention = max(1,min(30,int(retention_days)))*86400
        self.account_alias = account_alias
        self.quota_get = quota_get
        self.private.mkdir(mode=0o700, parents=True, exist_ok=True)
        self.spool.mkdir(mode=0o750, parents=True, exist_ok=True)
        self.state_path = self.private / 'state.json'
        try:
            self.state = bounded_json(self.state_path, 8*1024*1024)
        except FileNotFoundError:
            self.state = {'collector_id': str(uuid.uuid4()), 'baseline': {}, 'last': None, 'outbox': []}

    def read_metrics(self):
        uri = self.database.resolve().as_uri() + '?mode=ro'
        with contextlib.closing(sqlite3.connect(uri, uri=True, timeout=0.2)) as db:
            db.execute('PRAGMA query_only=ON')
            deadline=time.monotonic()+0.25
            operations=0
            def budget():
                nonlocal operations
                operations+=1000
                return int(operations>250000 or time.monotonic()>deadline)
            db.set_progress_handler(budget,1000)
            db.execute('BEGIN')
            # Schema metadata only; optional JOIN reads IDs + rewind_count, never content.
            columns = {r[1] for r in db.execute('PRAGMA table_info(sessions)')}
            query = METRIC_SQL
            if 'rewind_count' in columns:
                query = query.replace('u.first_seen,u.last_seen', 'u.first_seen,u.last_seen,COALESCE(s.rewind_count,0)').replace('FROM session_model_usage u', 'FROM session_model_usage u LEFT JOIN sessions s ON s.id=u.session_id')
            rows = db.execute(query).fetchall()
            db.rollback()
        if len(rows) > 4096:
            raise ValueError('metric cardinality exceeds local limit')
        return rows

    def quota_identity(self):
        if self.auth_path is None:
            return None
        store = bounded_json(self.auth_path, 1024*1024)
        tokens = store.get('providers',{}).get('openai-codex',{}).get('tokens',{})
        token = tokens.get('access_token')
        if not isinstance(token,str) or len(token)>32768:
            return None
        segment = token.split('.')[1]
        claims = json.loads(base64.urlsafe_b64decode(segment+'='*(-len(segment)%4)))
        expiry = claims.get('exp')
        account = tokens.get('account_id') or claims.get('https://api.openai.com/auth',{}).get('chatgpt_account_id')
        identity = account or self.account_alias
        if not isinstance(identity,str) or not identity or len(identity)>256:
            return None
        return token, account, expiry, hashlib.sha256(('komari:openai-codex:account:'+identity).encode()).hexdigest()

    def read_quota(self, now):
        result = {'provider':'openai-codex','status':'unavailable','observed_at':float(now),'windows':[]}
        try:
            identity = self.quota_identity()
            if identity is None:
                return result
            token, account, expiry, account_key = identity
            result['account_key'] = account_key
            if not isinstance(expiry,(int,float)) or not math.isfinite(expiry) or expiry<=now+30:
                return result
            headers={'Authorization':'Bearer '+token,'Accept':'application/json'}
            if account:
                headers['ChatGPT-Account-ID']=account
            request = urllib.request.Request('https://chatgpt.com/backend-api/wham/usage',headers=headers,method='GET')
            payload = self.quota_get(request)
            windows=[]
            rate=payload.get('rate_limit') or {}
            for key in ('primary_window','secondary_window'):
                window=rate.get(key)
                if not isinstance(window,dict):
                    continue
                seconds=window.get('limit_window_seconds')
                used=window.get('used_percent')
                if isinstance(used,bool) or not isinstance(used,(int,float)) or not math.isfinite(used) or not 0<=used<=100:
                    used=None
                row={'label':{18000:'Session',604800:'Weekly'}.get(seconds,'unknown'),'used_percent':used}
                if isinstance(seconds,(int,float)) and not isinstance(seconds,bool) and math.isfinite(seconds) and 0<seconds<=366*86400:
                    row['window_seconds']=seconds
                reset=window.get('reset_at')
                if isinstance(reset,(int,float)) and not isinstance(reset,bool) and math.isfinite(reset) and 0<reset<32503680000:
                    row['resets_at']=datetime.datetime.fromtimestamp(reset,datetime.timezone.utc).isoformat()
                windows.append(row)
            if not any(window['used_percent'] is not None for window in windows):
                return result
            result.update(status='available',last_success_at=float(now),plan=identifier(payload.get('plan_type')),windows=windows)
        except (OSError,ValueError,KeyError,IndexError,TypeError,AttributeError,OverflowError):
            # Never log exceptions containing OAuth material or raw responses.
            pass
        return result

    def delivery_state(self):
        try:
            value=bounded_json(self.spool/'delivery.json',4096)
            allowed={'last_attempt_at','last_success_at','consecutive_failures','last_error_class'}
            if not isinstance(value,dict) or set(value)-allowed:
                return None
            if value.get('last_error_class','') not in ('none','network','timeout','http_4xx','http_5xx','invalid_ack','spool_io'):
                return None
            if not isinstance(value.get('consecutive_failures',0),int) or isinstance(value.get('consecutive_failures'),bool) or not 0<=value.get('consecutive_failures',0)<=MAX_OUTBOX:
                return None
            return value
        except (OSError,ValueError,TypeError):
            return None

    def relay_capabilities(self):
        try:
            value=bounded_json(self.spool/'protocol.json',128)
            if isinstance(value,dict) and set(value)=={'version'} and value['version']==2:
                return 2
        except (OSError,ValueError,TypeError):
            pass
        return 1

    def tick(self, now):
        if not isinstance(now,(int,float)) or not math.isfinite(now):
            raise ValueError('finite sampling time required')
        try:
            control = bounded_json(self.spool / 'control.json', 128)
            enabled = isinstance(control,dict) and set(control)=={'enabled'} and control['enabled'] is True
        except (OSError, ValueError):
            enabled = False
        last_success=self.state.get('last_success_at')
        if last_success is not None and now-last_success>self.retention:
            self.state['baseline']={}
            self.state['last']=None
        quota=self.state.get('quota')
        if quota and now-quota.get('last_success_at',quota.get('observed_at',0))>self.retention:
            self.state.pop('quota',None)
        ack_dir = self.spool / 'acks'
        ack_dir.mkdir(mode=0o750, exist_ok=True)
        accepted = {s['sample_id'] for s in self.state['outbox'] if (ack_dir/s['sample_id']).is_file()}
        # ACK removal is the only permitted deletion. Backpressure below halts
        # new sampling before the bounded outbox grows; retries keep the same
        # immutable IDs until the panel explicitly accepts them.
        self.state['outbox'] = [s for s in self.state['outbox'] if s['sample_id'] not in accepted]
        report = {'version': 1, 'collector_id': self.state['collector_id'], 'enabled': enabled,
                  'status': 'disabled', 'observed_at': float(now),
                  'sampling_interval_seconds': self.interval, 'samples': self.state['outbox'][:256]}
        if not enabled:
            self.state['last'] = None
        elif len(self.state['outbox']) >= MAX_OUTBOX:
            # Do not advance baselines or discard unacknowledged history when
            # the relay/panel is unavailable. This is a collection gap, not
            # zero usage, and ordinary Agent heartbeats remain independent.
            report['status'] = 'stale'
        else:
            old_quota=self.state.get('quota',{})
            try:
                identity=self.quota_identity()
                account_key=identity[3] if identity else None
            except (OSError,ValueError,KeyError,IndexError,TypeError,AttributeError,OverflowError):
                account_key=None
            if old_quota and old_quota.get('account_key') != account_key:
                # Account transitions invalidate cached entitlements before a
                # network refresh; never associate an old snapshot with a new
                # OAuth identity or an unidentifiable replacement.
                self.state.pop('quota',None)
                self.state.pop('quota_attempt_at',None)
                quota=None
            due = self.state.get('quota_attempt_at')
            if due is None or now-due>=self.quota_interval:
                self.state['quota_attempt_at']=float(now)
                atomic_json(self.state_path,self.state)  # Crash-safe >=900s network throttle.
                quota=self.read_quota(now)
                old_quota=self.state.get('quota',{})
                if quota['status']=='unavailable' and quota.get('account_key') and quota.get('account_key')==old_quota.get('account_key') and old_quota.get('last_success_at') and now-old_quota['last_success_at']<=self.retention:
                    quota={**old_quota,'status':'stale','observed_at':float(now)}
                self.state['quota']=quota
            if 'quota' in self.state:
                report['quota']=self.state['quota']
            try:
                rows = self.read_metrics()
                previous = self.state['baseline']
                baseline = {}
                groups = {}
                resets = {}
                start = self.state['last']
                if start is not None and not 0 < now-start <= max(60,self.interval*3):
                    start = None
                for row in rows:
                    key = hashlib.sha256(json.dumps(row[:6]).encode()).hexdigest()
                    if any(x is not None and (not isinstance(x,int) or isinstance(x,bool)) for x in row[6:12]):
                        raise ValueError('noninteger counters')
                    counts = [int(x or 0) for x in row[6:12]]
                    if any(x < 0 or x > 2**53 - 1 for x in counts):
                        raise ValueError('invalid counters')
                    rewind = int(row[14]) if len(row)>14 else 0
                    first_seen = float(row[12] or 0)
                    if not math.isfinite(first_seen):
                        raise ValueError('invalid row timestamp')
                    baseline[key] = {'counts':counts, 'rewind':rewind, 'first_seen':first_seen}
                    old = previous.get(key)
                    group = tuple(identifier(x) for x in (row[1],row[2],row[4],row[5] or 'main'))
                    if start is None:
                        continue
                    if old is None:
                        # Only a first_seen within the uninterrupted covered interval proves new usage.
                        if not start <= float(row[12] or 0) <= now:
                            resets[group]=resets.get(group,0)+1
                            groups.setdefault(group,[0]*6)
                            continue
                        old = {'counts':[0]*6,'rewind':rewind,'first_seen':first_seen}
                    # Recreated rows and older checkpoints lack a trustworthy
                    # common origin even when the new counters are larger.
                    if old.get('first_seen') != first_seen or old['rewind'] != rewind or any(a < b for a,b in zip(counts,old['counts'])):
                        resets[group]=resets.get(group,0)+1
                        groups.setdefault(group,[0]*6)
                        continue
                    delta = [a-b for a,b in zip(counts, old['counts'])]
                    totals = groups.setdefault(group, [0]*6)
                    for i, x in enumerate(delta):
                        totals[i] += x
                if len(groups)>256 or any(v>2**53-1 for counts in groups.values() for v in counts):
                    raise ValueError('aggregate cardinality exceeds local limit')
                if len(self.state['outbox']) + len(groups) > MAX_OUTBOX:
                    # Keep the baseline/last timestamp unchanged so a later
                    # accepted drain rebaselines rather than claiming a gap.
                    report['status']='stale'
                else:
                    for group, counts in groups.items():
                        sample = {'sample_id': str(uuid.uuid4()), 'started_at': float(start), 'ended_at': float(now),
                                  **dict(zip(('model','provider','billing_mode','task'),group)),
                                  **dict(zip(COUNTERS,counts)), 'coverage':group not in resets, 'reset_count':resets.get(group,0)}
                        self.state['outbox'].append(sample)
                    self.state['baseline'] = baseline
                    self.state['last'] = float(now)
                    self.state['last_success_at'] = float(now)
                    report['status'] = 'collecting' if resets or any(any(c) for c in groups.values()) else 'no_usage'
            except (sqlite3.Error, OSError, ValueError):
                report['status'] = 'database_unavailable' if self.database.exists() else 'hermes_not_detected'
                self.state['last'] = None
        if 'last_success_at' in self.state:
            report['last_success_at'] = self.state['last_success_at']
        atomic_json(self.state_path, self.state)
        # Persist removal before deleting ACKs; a crash may replay an ACK but never a sample.
        pending={s['sample_id'] for s in self.state['outbox']}
        with os.scandir(ack_dir) as entries:
            for entry in itertools.islice(entries,8192):
                # Orphan ACKs from a helper/relay race have no further purpose.
                if entry.is_file(follow_symlinks=False) and entry.name not in pending:
                    pathlib.Path(entry.path).unlink(missing_ok=True)
        report['samples'] = self.state['outbox'][:256]
        # Existing panels decode report version 1 strictly. Only include the
        # additive delivery fields after the local Agent explicitly advertises
        # the matching capability; an old Agent leaves the legacy wire shape.
        if self.relay_capabilities() >= 2:
            report['pending_sample_count']=len(self.state['outbox'])
            if self.state['outbox']:
                report['oldest_pending_ended_at']=min(float(s.get('ended_at',now)) for s in self.state['outbox'])
            delivery=self.delivery_state()
            if delivery is not None:
                report['delivery']=delivery
        atomic_json(self.spool / 'report.json', report, 0o640)
        return report


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--database',required=True)
    parser.add_argument('--state-dir',required=True)
    parser.add_argument('--spool',required=True)
    parser.add_argument('--auth-path')
    parser.add_argument('--account-alias')
    parser.add_argument('--interval',type=float,default=10)
    parser.add_argument('--quota-interval',type=float,default=900)
    parser.add_argument('--retention-days',type=int,default=30)
    parser.add_argument('--once',action='store_true')
    args=parser.parse_args()
    if not all(math.isfinite(x) for x in (args.interval,args.quota_interval)):
        parser.error('intervals must be finite')
    import fcntl  # Linux helper only; do not claim unsupported platforms work.
    os.umask(0o027)
    private=pathlib.Path(args.state_dir)
    private.mkdir(mode=0o700,parents=True,exist_ok=True)
    with (private/'collector.lock').open('a') as lock:
        try:
            fcntl.flock(lock,fcntl.LOCK_EX|fcntl.LOCK_NB)
        except BlockingIOError:
            parser.exit(1,'collector already running\n')
        collector=Collector(args.database,args.state_dir,args.spool,args.interval,args.auth_path,args.quota_interval,args.retention_days,args.account_alias)
        while True:
            try:
                collector.tick(time.time())
            except (OSError,ValueError,TypeError):
                # A broken state/spool is a service failure, not synthetic zero usage.
                parser.exit(1,'collector state unavailable\n')
            if args.once:
                break
            time.sleep(collector.interval)


if __name__=='__main__':
    main()
