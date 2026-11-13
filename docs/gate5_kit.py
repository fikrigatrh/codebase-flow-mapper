# Gate 5 kit (PRD §6.5): picks 6 endpoints (seed 305) from a cfm --json graph, splits them into set A
# (answered without the report) and set B (with it), and prints the answer key as Markdown.
# Usage: python3 docs/gate5_kit.py graph.json
import json, random, sys
from collections import deque

g = json.load(open(sys.argv[1]))
by = {n['id']: n for n in g['nodes']}
out = {}
for e in g['edges']:
    out.setdefault(e['source'], []).append(e)

def isrepo(n):
    return n['kind'] == 'function' and any(p.endswith(('repo', 'repository')) for p in n.get('pkg', '').split('/'))

def short(n):
    return n['pkg'].rsplit('/', 1)[-1] + '.' + n['name']

def reach(ep, stop=True):
    # ponytail: low-confidence edges are skipped (err.Error() fan-out); a call reached only that way is missed.
    seen, q, hits = {ep}, deque([ep]), set()
    while q:
        x = q.popleft()
        if isrepo(by[x]):
            hits.add(x)
            if stop:
                continue
        for e in out.get(x, []):
            if e['kind'] == 'uses_middleware' or e['confidence'] == 'low' or e['target'] in seen:
                continue
            seen.add(e['target'])
            q.append(e['target'])
    return seen, hits

def answer(ep):
    handler = next(e['target'] for e in out[ep] if e['kind'] == 'routes_to')
    seen, hits = reach(ep)
    # Q1: what the handler hands the work to, through an interface (the service/usecase layer).
    q1 = sorted({short(by[e['target']]) for e in out.get(handler, [])
                 if e['method'] == 'vta' and by[e['target']]['kind'] == 'function'})
    cats = sorted({by[x].get('category') for x in reach(ep, stop=False)[0] if by[x]['kind'] == 'external'} & {'db', 'http_client', 'cache'})
    return handler, q1, sorted(short(by[h]) for h in hits), cats

eps = sorted(n['id'] for n in g['nodes'] if n['kind'] == 'endpoint'
             and any(e['kind'] == 'routes_to' for e in out.get(n['id'], [])))
cands = [ep for ep in eps if 2 <= len(reach(ep)[1]) <= 10]  # enough to ask about, small enough to answer by hand
random.seed(305)
pick = random.sample(cands, 6)
for label, part in (('A (without the report)', pick[0::2]), ('B (with the report)', pick[1::2])):
    print(f'### Set {label}\n')
    for ep in part:
        handler, q1, repos, cats = answer(ep)
        n = by[ep]
        print(f"- **`{n['name']}`** ({n['loc']['file']}:{n['loc']['line']}), handler `{short(by[handler])}`")
        print(f"  - Q1: {', '.join(f'`{x}`' for x in q1) or 'none (no interface call)'}")
        print(f"  - Q2 ({len(repos)}): {', '.join(f'`{x}`' for x in repos)}")
        print(f"  - Q3: {', '.join(cats) or 'none'}")
    print()
