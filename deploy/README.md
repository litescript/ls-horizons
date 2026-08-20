# Deploying ls-horizons as a data endpoint

`ls-horizons` has no HTTP server. In serve mode it writes two JSON files on a
timer; a web server hands those files to browsers. The whole design turns on one
property:

> **The rate at which NASA and JPL are contacted is a function of time only —
> never of how many people visit your site.**

One visitor or ten thousand, upstream sees the same one request per minute.
Keep that property and you are a good citizen by construction rather than by
good intentions.

---

## The two endpoints

| File | Contents | Changes |
|---|---|---|
| `dsn.json` | Live Deep Space Network state: stations, antennas, active links, per-complex load | Every poll |
| `solarsystem.json` | Heliocentric positions for the Sun, eight planets, and any range-resolved spacecraft | Continuously, but slowly |

`solarsystem.json` uses **J2000 heliocentric ecliptic coordinates in AU**. The Sun
is at the origin and the ecliptic is the XY plane, so the coordinates drop
straight into a 3D scene:

```js
const res  = await fetch('/api/solarsystem.json');
const data = await res.json();

if (!data.schema_version.startsWith('1.')) {
  throw new Error(`unsupported schema ${data.schema_version}`);
}

const AU = 50; // scene units per AU
for (const body of data.bodies) {
  mesh(body.code).position.set(
    body.position.x * AU,
    body.position.z * AU,  // three.js is Y-up; ecliptic Z becomes scene Y
   -body.position.y * AU,  // negated, see "Axis convention" below
  );
}
```

### Axis convention

The payload uses a right-handed J2000 ecliptic frame: `+X` toward the vernal
equinox, `+Z` toward the north ecliptic pole. three.js is also right-handed but
Y-up, so the mapping is:

```
scene.x =  ecliptic.x
scene.y =  ecliptic.z
scene.z = -ecliptic.y
```

The negation is not cosmetic. Mapping straight to `(x, z, y)` swaps two axes,
which flips the handedness and renders the whole scene mirror-imaged, with the
planets orbiting backwards.

Each body carries a `source` field: `keplerian` for locally propagated planetary
orbits, `dsn` for live-tracked spacecraft, `static` for the Sun.

### Spacecraft appear only when DSN publishes ranging

A spacecraft can only be placed in 3D if the feed gives a range. The DSN feed
frequently reports `-1` for `uplegRange`, `downlegRange`, and `rtlt` across every
target at once — during those periods `solarsystem.json` contains the Sun and
planets only, and `dsn.json` reports `null` for `distance_km` and `rtlt_seconds`.

This is upstream data availability, not a fault in the pipeline. Build the scene
to tolerate an empty spacecraft list rather than assuming one.

---

## Install

Build and place the binary:

```sh
go build -o /usr/local/bin/ls-horizons ./cmd/ls-horizons
```

Create an unprivileged user:

```sh
sudo useradd --system --no-create-home --shell /usr/sbin/nologin ls-horizons
```

Install and start the unit:

```sh
sudo cp deploy/ls-horizons-snapshot.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now ls-horizons-snapshot.service
```

`StateDirectory=ls-horizons` creates `/var/lib/ls-horizons` with the right
ownership automatically. Confirm it is publishing:

```sh
systemctl status ls-horizons-snapshot
ls -la /var/lib/ls-horizons/web/
jq '.generator, .fetched_at, (.links | length)' /var/lib/ls-horizons/web/dsn.json
```

Then point Caddy at that directory using `deploy/Caddyfile.example`.

### Timer instead of a daemon

If you would rather not keep a process resident, drop `--watch` and drive it from
a `systemd` timer — `--serve-dir` writes once and exits:

```ini
# ls-horizons-snapshot.service
[Service]
Type=oneshot
ExecStart=/usr/local/bin/ls-horizons --serve-dir /var/lib/ls-horizons/web

# ls-horizons-snapshot.timer
[Timer]
OnBootSec=1min
OnUnitActiveSec=1min
RandomizedDelaySec=15
```

Set `RandomizedDelaySec` so the timer doesn't fire on an exact wall-clock
boundary. The daemon path jitters internally; the timer path needs to be told.

---

## Being a good citizen upstream

Both upstreams are public science services run on public money. Neither charges,
neither requires a key, and both can be degraded by thoughtless clients.

**NASA DSN** (`eyes.nasa.gov/dsn/data/dsn.xml`) is a static XML file behind
CloudFront, regenerated about every five seconds. The client:

- revalidates with `ETag` / `If-Modified-Since`, so an unchanged feed transfers
  zero bytes;
- identifies itself with a real version and a project URL, so someone can make
  contact instead of silently blocking a mystery agent;
- honors `Retry-After` on 429 and 503, and pauses process-wide when asked to;
- retries with jittered exponential backoff, and never retries a 4xx;
- jitters every poll interval so instances don't synchronize.

At the recommended 60s cadence that is 1,440 requests/day of roughly 4 KB —
comfortably below NASA's own DSN Now web client, which polls at 5s.

**JPL Horizons** (`ssd.jpl.nasa.gov/api/horizons.api`) is a live computation
engine, not a cached file, and it has a published fair use policy:

> *"You agree to submit only one API request at a time (no simultaneous
> requests)."*
>
> *"You may not embed these APIs in your website (per NASA CORS policy)."*

Both are honored:

- **One at a time.** Every Horizons request in the process passes through a
  single gate that holds its lock across the network call, with at least a
  second between consecutive requests.
- **Not embedded.** Horizons is never reachable from the browser and is never
  proxied per-visitor. Planet positions are propagated locally from orbital
  elements — accurate to a few arcminutes, and zero requests. Horizons is
  consulted only for on-demand spacecraft work in the interactive TUI, cached
  for five minutes.

**Do not** reverse-proxy Horizons to work around its missing CORS headers. The
absent header *is* the policy being enforced. Serving your own cached, derived
snapshot is a different thing and is what this deployment does.

If you scale this up meaningfully — many instances, or a much shorter interval —
contact JPL and NASA first rather than assuming the defaults still apply.
