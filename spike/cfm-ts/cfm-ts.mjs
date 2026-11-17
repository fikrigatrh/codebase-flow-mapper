// cfm-ts: throwaway spike for CFM-502 (ADR-108). Reads a NestJS project with the TypeScript
// compiler API and writes a GRAPH_SCHEMA 2.x graph for `cfm render`. Never runs target code:
// the compiler only parses and type-checks.
//
// usage: node cfm-ts.mjs <project dir> [-o graph.json]
import fs from 'node:fs';
import path from 'node:path';
import ts from 'typescript';

const VERBS = { Get: 'GET', Post: 'POST', Put: 'PUT', Patch: 'PATCH', Delete: 'DELETE', Head: 'HEAD', Options: 'OPTIONS', All: 'ANY' };
const NEST = '@nestjs/common';
const MAX_TEXT = 120; // same cap as the Go analyzer for source text in the report

export function analyze(dir) {
  const start = Date.now();
  const root = path.resolve(dir);
  const program = load(root);
  const checker = program.getTypeChecker();
  const own = program.getSourceFiles().filter((f) => !f.isDeclarationFile && !f.fileName.includes('/node_modules/'));
  const rel = (file) => path.relative(root, file).split(path.sep).join('/');
  const nodes = new Map();
  const edges = [];
  const diagnostics = [];

  const sym = (n) => {
    let s = n && checker.getSymbolAtLocation(n);
    if (s && s.flags & ts.SymbolFlags.Alias) s = checker.getAliasedSymbol(s);
    return s;
  };
  const text = (n) => n.getText().replace(/\s+/g, ' ').slice(0, MAX_TEXT);
  const locOf = (n, withEnd) => {
    const sf = n.getSourceFile();
    const s = sf.getLineAndCharacterOfPosition(n.getStart());
    const loc = { file: rel(sf.fileName), line: s.line + 1, col: s.character + 1 };
    if (withEnd) loc.endLine = sf.getLineAndCharacterOfPosition(n.getEnd()).line + 1;
    return loc;
  };
  const isOwn = (sf) => own.includes(sf);
  const modPath = (sf) => rel(sf.fileName).replace(/\.[cm]?tsx?$/, '');
  const decorator = (n, names) =>
    (ts.canHaveDecorators(n) ? ts.getDecorators(n) ?? [] : []).find(
      (d) => ts.isCallExpression(d.expression) && ts.isIdentifier(d.expression.expression) && names.includes(d.expression.expression.text) && importsFrom(n.getSourceFile(), d.expression.expression.text, NEST),
    );
  const classes = own.flatMap((sf) => sf.statements.filter((s) => ts.isClassDeclaration(s) && s.name));

  // --- function ids -------------------------------------------------------------------------
  const fnId = (decl) => {
    const sf = decl.getSourceFile();
    const cls = decl.parent && ts.isClassDeclaration(decl.parent) ? decl.parent.name.text : '';
    return 'fn:' + modPath(sf) + '.' + (cls ? `(${cls}).` : '') + decl.name.getText();
  };
  const addFunction = (decl) => {
    const id = fnId(decl);
    if (nodes.has(id)) return id; // ponytail: overloads share one implementation, so one node per name
    const cls = ts.isClassDeclaration(decl.parent) ? decl.parent.name.text : '';
    const sig = checker.getSignatureFromDeclaration(decl);
    nodes.set(id, {
      id, kind: 'function', name: (cls ? `(${cls}).` : '') + decl.name.getText(), pkg: modPath(decl.getSourceFile()),
      loc: locOf(decl, true), status: 'resolved', receiver: cls || undefined,
      signature: sig ? checker.signatureToString(sig).slice(0, MAX_TEXT) : undefined, decl,
    });
    return id;
  };
  const external = (pkg, name) => {
    const id = `external:${pkg}.${name}`;
    if (!nodes.has(id)) nodes.set(id, { id, kind: 'external', name, pkg, status: 'resolved', category: 'other' });
    return id;
  };
  const unresolved = (call, reason) => {
    const loc = locOf(call);
    const id = `unresolved:${loc.file}:${loc.line}:${loc.col}`;
    nodes.set(id, { id, kind: 'unresolved', name: text(call.expression), loc, status: 'unresolved', reason });
    return id;
  };

  // --- Nest providers: token symbol -> implementing class ----------------------------------
  const providers = new Map();
  for (const cls of classes) {
    const mod = decorator(cls, ['Module']);
    const meta = mod?.expression.arguments[0];
    if (!meta || !ts.isObjectLiteralExpression(meta)) continue;
    const list = prop(meta, 'providers');
    if (!list || !ts.isArrayLiteralExpression(list)) continue;
    for (const p of list.elements) {
      if (ts.isIdentifier(p)) {
        providers.set(sym(p), { cls: sym(p), token: p.text, impl: p.text, loc: locOf(p) });
      } else if (ts.isObjectLiteralExpression(p) && prop(p, 'provide') && prop(p, 'useClass')) {
        // ponytail: useFactory/useValue/useExisting are not followed; their calls fall back to the class hierarchy
        const tok = prop(p, 'provide');
        const impl = prop(p, 'useClass');
        providers.set(sym(tok), { cls: sym(impl), token: text(tok), impl: text(impl), loc: locOf(p) });
      }
    }
  }

  // The injection token of `this.x` when x is a constructor parameter property: @Inject(TOKEN), else its type.
  const injectionToken = (expr) => {
    if (!ts.isPropertyAccessExpression(expr) || expr.expression.kind !== ts.SyntaxKind.ThisKeyword) return undefined;
    const p = sym(expr.name)?.valueDeclaration;
    if (!p || !ts.isParameter(p) || !ts.isConstructorDeclaration(p.parent)) return undefined;
    const inject = decorator(p, ['Inject']);
    if (inject) return sym(inject.expression.arguments[0]);
    return p.type && ts.isTypeReferenceNode(p.type) ? sym(p.type.typeName) : undefined;
  };
  const classDecl = (s) => s?.declarations?.find(ts.isClassDeclaration);
  const findMethod = (cls, name) => {
    for (let c = cls; c; ) {
      const m = c.members.find((x) => ts.isMethodDeclaration(x) && x.body && x.name.getText() === name);
      if (m) return m;
      const base = c.heritageClauses?.find((h) => h.token === ts.SyntaxKind.ExtendsKeyword)?.types[0];
      c = base && classDecl(sym(base.expression));
    }
    return undefined;
  };
  // ponytail: direct `extends`/`implements` only; a deeper hierarchy is missed.
  const implementers = (typeSym) =>
    classes.filter((c) => c.heritageClauses?.some((h) => h.types.some((t) => sym(t.expression) === typeSym)));

  // --- call resolution ----------------------------------------------------------------------
  const resolve = (call) => {
    const callee = call.expression;
    const decl = checker.getResolvedSignature(call)?.declaration;
    if (!decl || !decl.getSourceFile) {
      const pkg = importedPackage(call.getSourceFile(), rootIdent(callee));
      if (pkg) return [{ to: external(pkg, shortName(callee)), method: 'static', confidence: 'high', reason: `call into ${pkg} (no type declarations installed)` }];
      return [{ to: unresolved(call, 'callee type unknown'), method: 'unresolved', confidence: 'unknown', reason: 'callee type unknown' }];
    }
    const sf = decl.getSourceFile();
    if (!isOwn(sf)) {
      const pkg = sf.fileName.match(/node_modules\/((?:@[^/]+\/)?[^/]+)/)?.[1] ?? 'lib';
      return [{ to: external(pkg, externalName(callee)), method: 'static', confidence: 'high', reason: `call into ${pkg}` }];
    }
    const name = decl.name?.getText();
    const token = injectionToken(ts.isPropertyAccessExpression(callee) ? callee.expression : callee);
    const provider = token && providers.get(token);
    if (provider) {
      const m = findMethod(classDecl(provider.cls), name);
      if (m) {
        const how = provider.token === provider.impl ? provider.impl : `{ provide: ${provider.token}, useClass: ${provider.impl} }`;
        // ponytail: a provider whose token is the class itself is plain DI of that class; it is reported
        // as static so the report matches what a reader expects from `this.svc.create()`.
        if (provider.token === provider.impl) return [{ to: addFunction(m), method: 'static', confidence: 'high', reason: 'static call' }];
        return [{ to: addFunction(m), method: 'framework_registration', confidence: 'medium', reason: `Nest provider ${how} at ${provider.loc.file}:${provider.loc.line}` }];
      }
    }
    const abstract = ts.isMethodSignature(decl) || ts.getCombinedModifierFlags(decl) & ts.ModifierFlags.Abstract;
    if (!abstract && decl.body) return [{ to: addFunction(decl), method: 'static', confidence: 'high', reason: 'static call' }];
    const owner = decl.parent;
    const impls = implementers(sym(owner.name)).map((c) => findMethod(c, name)).filter(Boolean);
    if (impls.length === 0) return [{ to: unresolved(call, `no implementation of ${owner.name.text}.${name}`), method: 'unresolved', confidence: 'unknown', reason: 'no implementation found' }];
    const confidence = impls.length === 1 ? 'medium' : 'low';
    return impls.map((m) => ({
      to: addFunction(m), method: 'vta', confidence, alternatives: confidence === 'low' ? impls.length : undefined,
      reason: `class hierarchy: ${impls.length} implementation(s) of ${owner.name.text}, no Nest provider found`,
    }));
  };

  // `this.rows.set` reads as `Map.set`: a field name means nothing outside its class.
  const externalName = (callee) => {
    if (ts.isPropertyAccessExpression(callee) && ts.isPropertyAccessExpression(callee.expression) && callee.expression.expression.kind === ts.SyntaxKind.ThisKeyword) {
      return checker.typeToString(checker.getTypeAtLocation(callee.expression)).replace(/<.*$/, '') + '.' + callee.name.text;
    }
    return shortName(callee);
  };

  // --- flow outline (GRAPH_SCHEMA §5) and call edges ----------------------------------------
  const outline = (fnIdStr, stmts) => {
    const calls = (n, out) => {
      // ponytail: calls inside arrow functions and callbacks are attributed to the enclosing function
      ts.forEachChild(n, (c) => void calls(c, out)); // a truthy return would stop forEachChild early
      if (ts.isCallExpression(n)) out.push(callStep(fnIdStr, n));
      return out;
    };
    const block = (s) => (s ? (ts.isBlock(s) ? s.statements : [s]) : []);
    const steps = [];
    for (const s of stmts) {
      const line = locOf(s).line;
      if (ts.isIfStatement(s)) {
        steps.push(...calls(s.expression, []), { k: 'if', line, cond: text(s.expression), then: outline(fnIdStr, block(s.thenStatement)), else: outline(fnIdStr, block(s.elseStatement)) });
      } else if (ts.isReturnStatement(s)) {
        steps.push(...(s.expression ? calls(s.expression, []) : []), { k: 'return', line });
      } else if (ts.isThrowStatement(s)) {
        steps.push(...calls(s.expression, []), { k: 'panic', line });
      } else if (ts.isForStatement(s) || ts.isForOfStatement(s) || ts.isForInStatement(s) || ts.isWhileStatement(s) || ts.isDoStatement(s)) {
        const head = text(s).split('{')[0].trim();
        steps.push({ k: 'loop', line, head, body: outline(fnIdStr, block(s.statement)) });
      } else if (ts.isBlock(s)) {
        steps.push(...outline(fnIdStr, s.statements));
      } else if (ts.isTryStatement(s)) {
        // ponytail: try/catch/finally flattened in source order
        steps.push(...outline(fnIdStr, s.tryBlock.statements), ...outline(fnIdStr, s.catchClause?.block.statements ?? []), ...outline(fnIdStr, s.finallyBlock?.statements ?? []));
      } else if (ts.isSwitchStatement(s)) {
        steps.push(...calls(s.expression, []), {
          k: 'switch', line, tag: text(s.expression),
          cases: s.caseBlock.clauses.map((c) => ({ label: ts.isCaseClause(c) ? 'case ' + text(c.expression) : 'default', body: outline(fnIdStr, c.statements) })),
        });
      } else {
        steps.push(...calls(s, []));
      }
    }
    return steps;
  };
  const callStep = (from, call) => {
    const sf = call.getSourceFile();
    const p = sf.getLineAndCharacterOfPosition(call.arguments.pos - 1); // the "(" like Go's CallExpr.Lparen
    const site = { file: rel(sf.fileName), line: p.line + 1, col: p.character + 1 };
    const siteId = `${site.file}:${site.line}:${site.col}`;
    for (const t of resolve(call)) {
      edges.push({ source: from, target: t.to, kind: 'calls', confidence: t.confidence, method: t.method, reason: t.reason, site, siteId, alternatives: t.alternatives });
    }
    return { k: 'call', site: siteId, line: site.line };
  };

  // --- routes -------------------------------------------------------------------------------
  const globalPrefix = findGlobalPrefix(own);
  for (const cls of classes) {
    const ctl = decorator(cls, ['Controller']);
    if (!ctl) continue;
    const prefix = routeArg(ctl.expression.arguments[0]);
    for (const m of cls.members) {
      if (!ts.isMethodDeclaration(m) || !m.body) continue;
      const d = decorator(m, Object.keys(VERBS));
      if (!d) continue;
      const verb = VERBS[d.expression.expression.text];
      const sub = routeArg(d.expression.arguments[0]);
      const loc = locOf(d);
      if (prefix === undefined || sub === undefined) {
        diagnostics.push({ severity: 'warning', code: 'ROUTE_DYNAMIC', message: `non-literal route path in ${text(d)}`, loc });
        continue;
      }
      const p = '/' + [globalPrefix, prefix, sub].flatMap((x) => x.split('/')).filter(Boolean).join('/');
      const id = `endpoint:nest:${verb}:${p}:${loc.file}:${loc.line}`;
      nodes.set(id, { id, kind: 'endpoint', name: `${verb} ${p}`, loc, status: 'resolved', framework: 'nest', method: verb, path: p });
      edges.push({ source: id, target: addFunction(m), kind: 'routes_to', confidence: 'high', method: 'framework_registration', reason: `@Controller + @${d.expression.expression.text}`, site: loc });
    }
  }

  // Every function in the project gets a node and an outline, reachable or not, like the Go analyzer.
  for (const cls of classes) for (const m of cls.members) if (ts.isMethodDeclaration(m) && m.body) addFunction(m);
  for (const sf of own) for (const s of sf.statements) if (ts.isFunctionDeclaration(s) && s.body && s.name) addFunction(s);
  for (const n of [...nodes.values()]) {
    if (n.kind !== 'function') continue;
    n.flow = outline(n.id, n.decl.body.statements);
    delete n.decl;
  }

  diagnostics.push(...loadDiagnostics(program, own, locOf));
  const list = [...nodes.values()];
  return {
    schemaVersion: '2.1.0', analyzerVersion: 'cfm-ts-spike', module: readName(root), goVersion: '', generatedAt: '', editorRoot: null,
    nodes: list, edges, diagnostics,
    stats: {
      packages: own.length, functions: list.filter((n) => n.kind === 'function').length, endpoints: list.filter((n) => n.kind === 'endpoint').length,
      edges: edges.length, unresolved: list.filter((n) => n.kind === 'unresolved').length, elapsedMs: Date.now() - start, blockers: [],
    },
  };
}

function load(root) {
  const file = ts.findConfigFile(root, ts.sys.fileExists);
  if (!file) throw new Error(`${root}: no tsconfig.json`);
  const raw = ts.readConfigFile(file, ts.sys.readFile);
  const parsed = ts.parseJsonConfigFileContent(raw.config, ts.sys, path.dirname(file));
  return ts.createProgram(parsed.fileNames, { ...parsed.options, noEmit: true });
}

// Missing packages become one warning each, like GO_LOAD_ERROR; CFM never installs them.
function loadDiagnostics(program, own, locOf) {
  const out = [];
  const seen = new Set();
  for (const sf of own) {
    for (const d of program.getSemanticDiagnostics(sf)) {
      const msg = ts.flattenDiagnosticMessageText(d.messageText, ' ').slice(0, MAX_TEXT);
      const pkg = d.code === 2307 ? msg.match(/'([^']+)'/)?.[1] : undefined;
      const key = pkg ?? msg;
      if (seen.has(key)) continue;
      seen.add(key);
      const pos = sf.getLineAndCharacterOfPosition(d.start ?? 0);
      const loc = { file: locOf(sf).file, line: pos.line + 1, col: pos.character + 1 };
      out.push(pkg
        ? { severity: 'warning', code: 'TS_MODULE_MISSING', message: `package ${pkg} is not installed; its calls show without types`, loc, details: { fix: 'npm ci', package: pkg } }
        : { severity: 'warning', code: 'TS_TYPE_ERROR', message: msg, loc });
    }
  }
  return out;
}

function prop(obj, name) {
  return obj.properties.find((p) => ts.isPropertyAssignment(p) && p.name.getText() === name)?.initializer;
}

// '' for no argument, the literal for a string, undefined when the path is not a literal.
function routeArg(arg) {
  if (!arg) return '';
  if (ts.isStringLiteralLike(arg)) return arg.text;
  if (ts.isObjectLiteralExpression(arg)) return routeArg(prop(arg, 'path'));
  return undefined;
}

function findGlobalPrefix(files) {
  let found = '';
  const visit = (n) => {
    if (ts.isCallExpression(n) && ts.isPropertyAccessExpression(n.expression) && n.expression.name.text === 'setGlobalPrefix' && n.arguments[0] && ts.isStringLiteralLike(n.arguments[0])) found ||= n.arguments[0].text;
    ts.forEachChild(n, visit);
  };
  files.forEach(visit);
  return found;
}

function importsFrom(sf, name, module) {
  return sf.statements.some((s) => ts.isImportDeclaration(s) && s.moduleSpecifier.text === module && s.importClause?.namedBindings && ts.isNamedImports(s.importClause.namedBindings) && s.importClause.namedBindings.elements.some((e) => e.name.text === name));
}

function importedPackage(sf, ident) {
  if (!ident) return undefined;
  for (const s of sf.statements) {
    if (!ts.isImportDeclaration(s) || s.moduleSpecifier.text.startsWith('.')) continue;
    const b = s.importClause;
    if (b?.name?.text === ident || (b?.namedBindings && (ts.isNamespaceImport(b.namedBindings) ? b.namedBindings.name.text === ident : b.namedBindings.elements.some((e) => e.name.text === ident)))) return s.moduleSpecifier.text;
  }
  return undefined;
}

function rootIdent(e) {
  while (ts.isPropertyAccessExpression(e) || ts.isCallExpression(e)) e = e.expression;
  return ts.isIdentifier(e) ? e.text : undefined;
}

function shortName(callee) {
  return callee.getText().replace(/^this\./, '').replace(/\s+/g, '').slice(0, MAX_TEXT);
}

function readName(root) {
  try {
    return JSON.parse(fs.readFileSync(path.join(root, 'package.json'), 'utf8')).name || path.basename(root);
  } catch {
    return path.basename(root);
  }
}

if (import.meta.url === `file://${process.argv[1]}`) {
  const [dir, flag, out] = process.argv.slice(2);
  if (!dir || (flag && flag !== '-o')) {
    console.error('usage: node cfm-ts.mjs <project dir> [-o graph.json]');
    process.exit(3);
  }
  const json = JSON.stringify(analyze(dir), null, 2) + '\n';
  if (out) fs.writeFileSync(out, json);
  else process.stdout.write(json);
}
