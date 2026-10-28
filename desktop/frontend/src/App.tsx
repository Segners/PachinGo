import {FormEvent, useCallback, useEffect, useRef, useState} from 'react';
import './App.css';
import logo from './assets/pachingo.png';
import {
    ApplyUpdates,
    CheckUpdates,
    DeleteTarget,
    ExportExcel,
    GetAuditTail,
    GetConfigPath,
    ListTargets,
    ScanInventory,
    SelectConfigFile,
    UpsertTarget,
} from '../wailsjs/go/main/App';
import {EventsOn} from '../wailsjs/runtime/runtime';

type Target = {
    name: string;
    kind: string;
    host: string;
    user?: string;
    port?: number;
    private_key?: string;
    package_manager?: string;
    tags?: string[];
    enabled?: boolean;
};

type HostResult = {
    name: string;
    kind: string;
    host: string;
    os?: string;
    updates?: { package: string; current_version: string; available_version: string }[];
    reboot_required: boolean;
    error?: string;
};

type HostInventory = {
    name: string;
    kind: string;
    host: string;
    os?: string;
    techs: { name: string; version: string; source: string; path?: string }[];
    error?: string;
};

type AuditEvent = {
    time: string;
    user: string;
    action: string;
    target?: string;
    success: boolean;
    detail?: string;
};

type Tab = 'targets' | 'patch' | 'inventory' | 'export';

/** Liste cochable des cibles activées, avec raccourcis VM / LXC. */
function TargetPicker({
    targets,
    selected,
    onChange,
    disabled,
}: {
    targets: Target[];
    selected: string[];
    onChange: (names: string[]) => void;
    disabled?: boolean;
}) {
    const enabled = targets.filter((t) => t.enabled !== false);
    const selectedSet = new Set(selected);

    function apply(kind: 'all' | 'none' | 'vm' | 'lxc') {
        if (kind === 'all') {
            onChange(enabled.map((t) => t.name));
            return;
        }
        if (kind === 'none') {
            onChange([]);
            return;
        }
        onChange(enabled.filter((t) => t.kind === kind).map((t) => t.name));
    }

    function toggle(name: string) {
        if (selectedSet.has(name)) {
            onChange(selected.filter((n) => n !== name));
            return;
        }
        onChange([...selected, name]);
    }

    return (
        <div className="picker">
            <div className="btn-row">
                <button type="button" className="ghost" disabled={disabled} onClick={() => apply('all')}>
                    Tout
                </button>
                <button type="button" className="ghost" disabled={disabled} onClick={() => apply('none')}>
                    Aucun
                </button>
                <button type="button" className="ghost" disabled={disabled} onClick={() => apply('vm')}>
                    VM
                </button>
                <button type="button" className="ghost" disabled={disabled} onClick={() => apply('lxc')}>
                    LXC
                </button>
            </div>
            {enabled.length === 0 ? (
                <p className="muted">Aucune cible activée.</p>
            ) : (
                <ul className="picker-list">
                    {enabled.map((t) => (
                        <li key={t.name}>
                            <label>
                                <input
                                    type="checkbox"
                                    checked={selectedSet.has(t.name)}
                                    disabled={disabled}
                                    onChange={() => toggle(t.name)}
                                />
                                <span>{t.name}</span>
                                <span className="kind">{t.kind}</span>
                                <span className="muted">{t.host}</span>
                            </label>
                        </li>
                    ))}
                </ul>
            )}
        </div>
    );
}

const emptyForm: Target & { password?: string } = {
    name: '',
    kind: 'vm',
    host: '',
    user: '',
    port: 22,
    private_key: '~/.ssh/id_ed25519',
    package_manager: '',
    tags: [],
    enabled: true,
    password: '',
};

function App() {
    const [tab, setTab] = useState<Tab>('targets');
    const [configPath, setConfigPath] = useState('');
    const [targets, setTargets] = useState<Target[]>([]);
    const [form, setForm] = useState(emptyForm);
    const [editing, setEditing] = useState(false);
    const [busy, setBusy] = useState(false);
    const [status, setStatus] = useState('');
    const [checks, setChecks] = useState<HostResult[]>([]);
    const [inventory, setInventory] = useState<HostInventory[]>([]);
    const [exportPath, setExportPath] = useState('');
    const [audit, setAudit] = useState<AuditEvent[]>([]);
    const [tagsInput, setTagsInput] = useState('');
    const [selectedPatchTarget, setSelectedPatchTarget] = useState<string | null>(null);
    const [consoleByTarget, setConsoleByTarget] = useState<Record<string, string>>({});
    const [patchSel, setPatchSel] = useState<string[]>([]);
    const [invSel, setInvSel] = useState<string[]>([]);
    const consoleRef = useRef<HTMLPreElement | null>(null);
    const selReady = useRef(false);

    const refresh = useCallback(async () => {
        const path = await GetConfigPath();
        setConfigPath(path);
        const list = await ListTargets();
        setTargets(list ?? []);
        const events = await GetAuditTail(30);
        setAudit(events ?? []);
    }, []);

    useEffect(() => {
        refresh().catch((e) => setStatus(String(e)));
    }, [refresh]);

    useEffect(() => {
        const enabled = targets.filter((t) => t.enabled !== false).map((t) => t.name);
        if (!selReady.current) {
            if (targets.length === 0) {
                return;
            }
            setPatchSel(enabled);
            setInvSel(enabled);
            selReady.current = true;
            return;
        }
        const allowed = new Set(enabled);
        setPatchSel((cur) => cur.filter((n) => allowed.has(n)));
        setInvSel((cur) => cur.filter((n) => allowed.has(n)));
    }, [targets]);

    useEffect(() => {
        // Hors de Wails (vite seul), le runtime n'existe pas : on n'écoute pas les logs SSH.
        if (!('runtime' in window)) {
            return;
        }
        const off = EventsOn('patch:log', (payload: { target?: string; line?: string }) => {
            const target = payload?.target;
            const line = payload?.line ?? '';
            if (!target) {
                return;
            }
            setConsoleByTarget((prev) => ({
                ...prev,
                [target]: (prev[target] ? prev[target] + '\n' : '') + line,
            }));
            setSelectedPatchTarget((cur) => cur ?? target);
        });
        return () => {
            off();
        };
    }, []);

    useEffect(() => {
        if (consoleRef.current) {
            consoleRef.current.scrollTop = consoleRef.current.scrollHeight;
        }
    }, [consoleByTarget, selectedPatchTarget]);

    async function pickConfig() {
        try {
            const path = await SelectConfigFile();
            if (path) {
                setConfigPath(path);
                await refresh();
                setStatus('Config chargée');
            }
        } catch (e) {
            setStatus(String(e));
        }
    }

    function startEdit(t: Target) {
        setEditing(true);
        setForm({
            ...emptyForm,
            ...t,
            port: t.port || 22,
            password: '',
        });
        setTagsInput((t.tags ?? []).join(', '));
    }

    function startCreate() {
        setEditing(false);
        setForm({...emptyForm});
        setTagsInput('');
    }

    async function onSave(e: FormEvent) {
        e.preventDefault();
        setBusy(true);
        try {
            const tags = tagsInput
                .split(',')
                .map((s) => s.trim())
                .filter(Boolean);
            const enabled = form.enabled !== false;
            await UpsertTarget(
                {
                    name: form.name.trim(),
                    kind: form.kind,
                    host: form.host.trim(),
                    user: form.user?.trim() || undefined,
                    port: form.port || 22,
                    private_key: form.private_key?.trim() || undefined,
                    package_manager: form.package_manager?.trim() || undefined,
                    tags,
                    enabled,
                },
                form.password?.trim() || '',
            );
            setStatus(editing ? 'Cible mise à jour' : 'Cible ajoutée au YAML');
            startCreate();
            await refresh();
        } catch (err) {
            setStatus(String(err));
        } finally {
            setBusy(false);
        }
    }

    async function onDelete(name: string) {
        if (!window.confirm(`Supprimer la cible « ${name} » du fichier config ?`)) {
            return;
        }
        setBusy(true);
        try {
            await DeleteTarget(name);
            setStatus('Cible supprimée');
            await refresh();
        } catch (err) {
            setStatus(String(err));
        } finally {
            setBusy(false);
        }
    }

    function selectedHosts(names: string[]) {
        const wanted = new Set(names);
        return targets.filter((t) => t.enabled !== false && wanted.has(t.name));
    }

    /** Vérifie les mises à jour des VM et LXC cochés. */
    async function runCheck() {
        if (patchSel.length === 0) {
            setStatus('Sélectionnez au moins une cible à vérifier');
            return;
        }
        setBusy(true);
        setStatus('Vérification des mises à jour…');
        setConsoleByTarget({});
        const placeholders = selectedHosts(patchSel).map((t) => ({
            name: t.name,
            kind: t.kind,
            host: t.host,
            updates: [],
            reboot_required: false,
        }));
        setChecks(placeholders);
        if (placeholders[0]) {
            setSelectedPatchTarget(placeholders[0].name);
        }
        try {
            const res = await CheckUpdates(patchSel);
            setChecks(res ?? []);
            setStatus('Check terminé');
            await refresh();
        } catch (err) {
            setStatus(String(err));
        } finally {
            setBusy(false);
        }
    }

    /** Applique les upgrades sur les VM et LXC cochés, après confirmation. */
    async function runUpgrade() {
        if (patchSel.length === 0) {
            setStatus('Sélectionnez au moins une cible à mettre à jour');
            return;
        }
        if (!window.confirm(`Appliquer les upgrades sur ${patchSel.length} cible(s) ?`)) {
            return;
        }
        setBusy(true);
        setStatus('Upgrade en cours…');
        setConsoleByTarget({});
        const placeholders = selectedHosts(patchSel).map((t) => ({
            name: t.name,
            kind: t.kind,
            host: t.host,
            updates: [],
            reboot_required: false,
        }));
        setChecks(placeholders);
        if (placeholders[0]) {
            setSelectedPatchTarget(placeholders[0].name);
        }
        try {
            const res = await ApplyUpdates(true, patchSel);
            setChecks(res ?? []);
            setStatus('Upgrade terminé');
            await refresh();
        } catch (err) {
            setStatus(String(err));
        } finally {
            setBusy(false);
        }
    }

    /** Scanne l'inventaire des VM et LXC cochés. */
    async function runInventory() {
        if (invSel.length === 0) {
            setStatus('Sélectionnez au moins une cible à scanner');
            return;
        }
        setBusy(true);
        setStatus('Scan inventaire…');
        try {
            const res = await ScanInventory(invSel);
            setInventory(res ?? []);
            setStatus('Inventaire terminé');
            await refresh();
        } catch (err) {
            setStatus(String(err));
        } finally {
            setBusy(false);
        }
    }

    async function runExport() {
        setBusy(true);
        setStatus('Export Excel…');
        try {
            const path = await ExportExcel(true);
            setExportPath(path);
            setStatus('Excel généré');
            await refresh();
        } catch (err) {
            setStatus(String(err));
        } finally {
            setBusy(false);
        }
    }

    return (
        <div className="layout">
            <header className="top">
                <div className="brand-block">
                    <img className="logo" src={logo} alt="" />
                    <div>
                        <p className="brand">PachinGo</p>
                        <p className="sub">Patch & inventaire SSH — VM / LXC</p>
                    </div>
                </div>
                <nav className="nav" aria-label="Sections">
                    {([
                        ['targets', 'Cibles'],
                        ['patch', 'Patch'],
                        ['inventory', 'Inventaire'],
                        ['export', 'Export'],
                    ] as const).map(([id, label]) => (
                        <button
                            key={id}
                            type="button"
                            className={tab === id ? 'nav-btn active' : 'nav-btn'}
                            onClick={() => setTab(id)}
                        >
                            {label}
                        </button>
                    ))}
                </nav>
            </header>

            <p className="config-line">
                Config : <code>{configPath || '…'}</code>{' '}
                <button type="button" className="linkish" onClick={pickConfig}>
                    Changer…
                </button>
            </p>
            {status ? <p className="status" role="status">{status}</p> : null}

            <main className="main">
                {tab === 'targets' && (
                    <section className="panel">
                        <h1>Cibles</h1>
                        <p className="lead">Ajoutez ou modifiez les hôtes — écriture directe dans le YAML.</p>

                        <div className="split">
                            <div className="table-wrap">
                                <table>
                                    <thead>
                                    <tr>
                                        <th>Nom</th>
                                        <th>Kind</th>
                                        <th>Host</th>
                                        <th>User</th>
                                        <th></th>
                                    </tr>
                                    </thead>
                                    <tbody>
                                    {targets.length === 0 ? (
                                        <tr>
                                            <td colSpan={5}>Aucune cible — créez-en une.</td>
                                        </tr>
                                    ) : (
                                        targets.map((t) => (
                                            <tr key={t.name}>
                                                <td>{t.name}</td>
                                                <td>{t.kind}</td>
                                                <td>{t.host}</td>
                                                <td>{t.user || '—'}</td>
                                                <td className="actions">
                                                    <button type="button" onClick={() => startEdit(t)}>
                                                        Éditer
                                                    </button>
                                                    <button type="button" className="danger" onClick={() => onDelete(t.name)}>
                                                        Suppr.
                                                    </button>
                                                </td>
                                            </tr>
                                        ))
                                    )}
                                    </tbody>
                                </table>
                                <button type="button" className="secondary" onClick={startCreate}>
                                    Nouvelle cible
                                </button>
                            </div>

                            <form className="form" onSubmit={onSave}>
                                <h2>{editing ? 'Modifier' : 'Ajouter'}</h2>
                                <label>
                                    Nom
                                    <input
                                        required
                                        value={form.name}
                                        disabled={editing}
                                        onChange={(e) => setForm({...form, name: e.target.value})}
                                    />
                                </label>
                                <label>
                                    Kind
                                    <select
                                        value={form.kind}
                                        onChange={(e) => setForm({...form, kind: e.target.value})}
                                    >
                                        <option value="vm">vm</option>
                                        <option value="lxc">lxc</option>
                                    </select>
                                </label>
                                <label>
                                    Package manager
                                    <select
                                        value={form.package_manager || ''}
                                        onChange={(e) => setForm({...form, package_manager: e.target.value})}
                                    >
                                        <option value="">auto</option>
                                        <option value="apt">apt</option>
                                        <option value="dnf">dnf</option>
                                        <option value="yum">yum</option>
                                        <option value="apk">apk</option>
                                    </select>
                                </label>
                                <label>
                                    Host
                                    <input
                                        required
                                        value={form.host}
                                        onChange={(e) => setForm({...form, host: e.target.value})}
                                    />
                                </label>
                                <label>
                                    User
                                    <input
                                        value={form.user || ''}
                                        onChange={(e) => setForm({...form, user: e.target.value})}
                                    />
                                </label>
                                <label>
                                    Port
                                    <input
                                        type="number"
                                        value={form.port || 22}
                                        onChange={(e) => setForm({...form, port: Number(e.target.value)})}
                                    />
                                </label>
                                <label>
                                    Clé privée
                                    <input
                                        value={form.private_key || ''}
                                        onChange={(e) => setForm({...form, private_key: e.target.value})}
                                    />
                                </label>
                                <label>
                                    Mot de passe SSH/sudo (chiffré, pour sudo -S si besoin)
                                    <input
                                        type="password"
                                        autoComplete="new-password"
                                        value={form.password || ''}
                                        onChange={(e) => setForm({...form, password: e.target.value})}
                                    />
                                </label>
                                <label>
                                    Tags (virgules)
                                    <input value={tagsInput} onChange={(e) => setTagsInput(e.target.value)} />
                                </label>
                                <label className="check">
                                    <input
                                        type="checkbox"
                                        checked={form.enabled !== false}
                                        onChange={(e) => setForm({...form, enabled: e.target.checked})}
                                    />
                                    Activée
                                </label>
                                <button type="submit" disabled={busy}>
                                    Enregistrer dans le YAML
                                </button>
                            </form>
                        </div>
                    </section>
                )}

                {tab === 'patch' && (
                    <section className="panel">
                        <h1>Patch</h1>
                        <p className="lead">
                            Choisissez les VM et LXC à vérifier ou mettre à jour. Clique une ligne pour voir sa console SSH.
                        </p>
                        <TargetPicker
                            targets={targets}
                            selected={patchSel}
                            onChange={setPatchSel}
                            disabled={busy}
                        />
                        <div className="btn-row">
                            <button type="button" disabled={busy} onClick={runCheck}>
                                Check updates
                            </button>
                            <button type="button" className="warn" disabled={busy} onClick={runUpgrade}>
                                Upgrade (confirmé)
                            </button>
                        </div>
                        <div className="patch-split">
                            <div className="table-wrap">
                                <table>
                                    <thead>
                                    <tr>
                                        <th>Cible</th>
                                        <th>OS</th>
                                        <th>Updates</th>
                                        <th>Reboot</th>
                                        <th>Erreur</th>
                                    </tr>
                                    </thead>
                                    <tbody>
                                    {checks.length === 0 ? (
                                        <tr>
                                            <td colSpan={5}>Lancez un check.</td>
                                        </tr>
                                    ) : (
                                        checks.map((c) => (
                                            <tr
                                                key={c.name}
                                                className={selectedPatchTarget === c.name ? 'row-selected' : 'row-clickable'}
                                                onClick={() => setSelectedPatchTarget(c.name)}
                                            >
                                                <td>
                                                    {c.name} <span className="muted">({c.kind})</span>
                                                </td>
                                                <td>{c.os || '—'}</td>
                                                <td>{c.updates?.length ?? 0}</td>
                                                <td>{c.reboot_required ? 'oui' : 'non'}</td>
                                                <td className="err">{c.error || ''}</td>
                                            </tr>
                                        ))
                                    )}
                                    </tbody>
                                </table>
                            </div>
                            <aside className="console-panel" aria-label="Console SSH">
                                <div className="console-head">
                                    <span>Console</span>
                                    <code>{selectedPatchTarget || '—'}</code>
                                </div>
                                <pre className="console" ref={consoleRef}>
                                    {selectedPatchTarget
                                        ? consoleByTarget[selectedPatchTarget] ||
                                          (busy ? 'En attente de sortie…' : 'Pas encore de sortie pour cette cible.')
                                        : 'Sélectionne une cible dans le tableau.'}
                                </pre>
                            </aside>
                        </div>
                        {checks.some((c) => (c.updates?.length ?? 0) > 0) && (
                            <div className="table-wrap">
                                <h2>Détail paquets</h2>
                                <table>
                                    <thead>
                                    <tr>
                                        <th>Cible</th>
                                        <th>Paquet</th>
                                        <th>Actuel</th>
                                        <th>Dispo</th>
                                    </tr>
                                    </thead>
                                    <tbody>
                                    {checks.flatMap((c) =>
                                        (c.updates ?? []).map((u) => (
                                            <tr key={`${c.name}-${u.package}`}>
                                                <td>{c.name}</td>
                                                <td>{u.package}</td>
                                                <td>{u.current_version}</td>
                                                <td>{u.available_version}</td>
                                            </tr>
                                        )),
                                    )}
                                    </tbody>
                                </table>
                            </div>
                        )}
                    </section>
                )}

                {tab === 'inventory' && (
                    <section className="panel">
                        <h1>Inventaire</h1>
                        <p className="lead">
                            Choisissez les VM et LXC à scanner. La sélection est indépendante de celle du patch.
                        </p>
                        <TargetPicker
                            targets={targets}
                            selected={invSel}
                            onChange={setInvSel}
                            disabled={busy}
                        />
                        <div className="btn-row">
                            <button type="button" disabled={busy} onClick={runInventory}>
                                Scanner
                            </button>
                        </div>
                        <div className="table-wrap">
                            <table>
                                <thead>
                                <tr>
                                    <th>Host</th>
                                    <th>Kind</th>
                                    <th>Techno</th>
                                    <th>Version</th>
                                    <th>Source</th>
                                    <th>Path</th>
                                </tr>
                                </thead>
                                <tbody>
                                {inventory.length === 0 ? (
                                    <tr>
                                        <td colSpan={6}>Lancez un scan.</td>
                                    </tr>
                                ) : (
                                    inventory.flatMap((h) =>
                                        h.error
                                            ? [
                                                <tr key={`${h.name}-err`}>
                                                    <td>{h.name}</td>
                                                    <td>{h.kind}</td>
                                                    <td colSpan={4} className="err">
                                                        {h.error}
                                                    </td>
                                                </tr>,
                                            ]
                                            : h.techs.length === 0
                                                ? [
                                                    <tr key={`${h.name}-empty`}>
                                                        <td>{h.name}</td>
                                                        <td>{h.kind}</td>
                                                        <td colSpan={4} className="muted">
                                                            Aucune techno détectée
                                                        </td>
                                                    </tr>,
                                                ]
                                                : h.techs.map((t, i) => (
                                                    <tr key={`${h.name}-${t.name}-${t.path}-${i}`}>
                                                        <td>{h.name}</td>
                                                        <td>{h.kind}</td>
                                                        <td>{t.name}</td>
                                                        <td>{t.version}</td>
                                                        <td>{t.source}</td>
                                                        <td>{t.path || '—'}</td>
                                                    </tr>
                                                )),
                                    )
                                )}
                                </tbody>
                            </table>
                        </div>
                    </section>
                )}

                {tab === 'export' && (
                    <section className="panel">
                        <h1>Export Excel</h1>
                        <p className="lead">
                            Feuilles Hosts, Technologies et Updates. Utilise le dernier inventaire/check ou les
                            relance.
                        </p>
                        <div className="btn-row">
                            <button type="button" disabled={busy} onClick={runExport}>
                                Générer .xlsx
                            </button>
                        </div>
                        {exportPath ? (
                            <p>
                                Fichier : <code>{exportPath}</code>
                            </p>
                        ) : null}
                        <h2>Audit récent</h2>
                        <div className="table-wrap">
                            <table>
                                <thead>
                                <tr>
                                    <th>Heure</th>
                                    <th>User</th>
                                    <th>Action</th>
                                    <th>Cible</th>
                                    <th>OK</th>
                                    <th>Détail</th>
                                </tr>
                                </thead>
                                <tbody>
                                {audit.length === 0 ? (
                                    <tr>
                                        <td colSpan={6}>Pas encore d’événements.</td>
                                    </tr>
                                ) : (
                                    [...audit].reverse().map((e, i) => (
                                        <tr key={`${e.time}-${i}`}>
                                            <td>{e.time}</td>
                                            <td>{e.user}</td>
                                            <td>{e.action}</td>
                                            <td>{e.target || '—'}</td>
                                            <td>{e.success ? 'oui' : 'non'}</td>
                                            <td>{e.detail || ''}</td>
                                        </tr>
                                    ))
                                )}
                                </tbody>
                            </table>
                        </div>
                    </section>
                )}
            </main>
        </div>
    );
}

export default App;
