# Picks 5 endpoints (seed 301) from a cfm --json graph and prints the BFS path from each handler to
# repo-layer functions (a package path element ending in repo/repository). Used by docs/validation.md.
# Usage: python3 docs/sample_endpoints.py graph.json
import json, random, sys
from collections import deque
g = json.load(open(sys.argv[1])); by = {n['id']: n for n in g['nodes']}
out = {}
for e in g['edges']: out.setdefault(e['source'], []).append(e)
eps = sorted(n['id'] for n in g['nodes'] if n['kind'] == 'endpoint')
random.seed(301)
def isrepo(n): return n['kind'] == 'function' and any(p.endswith('repo') or p.endswith('repository') for p in n.get('pkg','').split('/'))
for ep in random.sample(eps, 5):
    print('==', by[ep]['name'], by[ep].get('loc',{}).get('file'), by[ep].get('loc',{}).get('line'))
    h = [e['target'] for e in out.get(ep, []) if e['kind'] == 'routes_to']
    seen = {ep: None}; q = deque([ep]); hits = []
    while q:
        x = q.popleft()
        if isrepo(by[x]): hits.append(x); continue
        for e in out.get(x, []):
            if e['kind'] == 'uses_middleware': continue
            if e['target'] not in seen: seen[e['target']] = (x, e); q.append(e['target'])
    print('  handler:', [by[t]['name'] + ' ' + by[t].get('pkg','') for t in h])
    print('  repo hits:', len(hits))
    for t in hits[:3]:
        path = []; y = t
        while seen[y]: x, e = seen[y]; path.append(f"{by[y]['name']} [{e['confidence']}/{e.get('method','')}] @{e.get('siteId','')}"); y = x
        print('   ', ' <- '.join(path))
    unres = [x for x in seen if by[x]['kind'] == 'unresolved']
    print('  unresolved reachable:', len(unres), [by[u].get('reason') for u in unres][:5])
fake = [n['id'] for n in g['nodes'] if any(w in (n['id']).lower() for w in ('mock', 'fake', 'stub'))]
tgt = {e['target'] for e in g['edges']}
print('GATE3 mock/fake/stub nodes:', len(fake), 'as targets:', [f for f in fake if f in tgt][:10])
