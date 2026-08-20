# Deploying ls-horizons as a data endpoint

`ls-horizons` has no HTTP server. In serve mode it writes JSON files on a
timer; a web server hands those files to browsers. The whole design turns on one
property:

> **The rate at which NASA and JPL are contacted is a function of time only —
> never of how many people visit your site.**

One visitor or ten thousand, upstream sees the same one request per minute.
Keep that property and you are a good citizen by construction rather than by
good intentions.

---

## The three endpoints

| File | Contents | Changes |
|---|---|---|
| `dsn.json` | Live Deep Space Network state: stations, antennas, active links, per-complex load | Every poll |
| `solarsystem.json` | Heliocentric positions for the Sun, eight planets, and any range-resolved spacecraft | Continuously, but slowly |
| `stars.json` | The naked-eye star catalog as celestial-sphere directions | Never |

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

Both payloads use a right-handed J2000 ecliptic frame: `+X` toward the vernal
equinox, `+Z` toward the north ecliptic pole. three.js is also right-handed but
Y-up, so the mapping is:

```
scene.x =  ecliptic.x
scene.y =  ecliptic.z
scene.z = -ecliptic.y
```

The negation is not cosmetic. Mapping straight to `(x, z, y)` swaps two axes,
which flips the handedness and renders the whole scene mirror-imaged: orbits run
backwards and, once you add `stars.json`, every constellation comes out
reversed. Use the same mapping for both payloads — mixing conventions puts the
planets and the sky in different universes.

Each body carries a `source` field: `keplerian` for locally propagated planetary
orbits, `dsn` for live-tracked spacecraft, `static` for the Sun.

### Spacecraft appear only when DSN publishes ranging

A spacecraft can only be placed in 3D if the feed gives a range. The DSN feed
frequently reports `-1` for `uplegRange`, `downlegRange`, and `rtlt` across every
target at once — during those periods `solarsystem.json` contains the Sun and
planets only, and `dsn.json` reports `null` for `distance_km` and `rtlt_seconds`.

This is upstream data availability, not a fault in the pipeline. Build the scene
to tolerate an empty spacecraft list rather than assuming one.

## Stars are static

`stars.json` is the odd one out. It holds 8,404 stars down to apparent visual
magnitude 6.5 — the naked-eye sky — from the Bright Star Catalogue, 5th Revised
Ed. (Hoffleit & Warren, 1991), and it **never changes**. The catalog is compiled
into the binary, so publishing it contacts nothing, and `--serve-dir` writes it
once at startup rather than on every poll even under `--watch`. Two runs of the
same binary emit identical bytes.

That makes it safe to cache hard:

```
@stars path /api/stars.json
header @stars Cache-Control "public, max-age=31536000, immutable"
```

Each record carries `ra_deg` and `dec_deg` for J2000, plus the same direction
precomputed as unit vectors in the `equatorial` and `ecliptic` frames. Use
`ecliptic` and the mapping above and the sky lines up with the planets.

**The vectors carry no distance.** They are unit length: this is a catalog of
directions on the celestial sphere, not a 3D map of where stars actually are.
Multiply by whatever shell radius your scene wants — far enough out that it
reads as a backdrop, near enough to stay inside your camera's far plane.

Colour is the raw B−V index in `bv`, not RGB, so the choice of how saturated a
sky you want stays yours. It is `null` for the 3% of stars with no published
photometry.

```js
const res  = await fetch('/api/stars.json');
const data = await res.json();

if (!data.schema_version.startsWith('1.')) {
  throw new Error(`unsupported schema ${data.schema_version}`);
}

const R = 5000; // celestial sphere radius, in scene units
const positions = new Float32Array(data.count * 3);
const colors    = new Float32Array(data.count * 3);
const sizes     = new Float32Array(data.count);

data.stars.forEach((s, i) => {
  // Ecliptic (right-handed, Z to the north ecliptic pole) -> three.js Y-up.
  positions[i * 3 + 0] =  s.ecliptic.x * R;
  positions[i * 3 + 1] =  s.ecliptic.z * R;
  positions[i * 3 + 2] = -s.ecliptic.y * R;

  const c = colorFromBV(s.bv);
  colors[i * 3 + 0] = c.r;
  colors[i * 3 + 1] = c.g;
  colors[i * 3 + 2] = c.b;

  // Magnitude is logarithmic and inverted: every 5 magnitudes is a factor of
  // 100 in flux. Rendering that linearly makes Sirius a dinner plate, so this
  // is a deliberately compressed curve, not photometry.
  sizes[i] = Math.max(0.6, 7 - s.mag);
});

const geometry = new THREE.BufferGeometry();
geometry.setAttribute('position', new THREE.BufferAttribute(positions, 3));
geometry.setAttribute('color',    new THREE.BufferAttribute(colors, 3));
geometry.setAttribute('size',     new THREE.BufferAttribute(sizes, 1));

scene.add(new THREE.Points(geometry, new THREE.PointsMaterial({
  vertexColors: true,
  sizeAttenuation: false,
  depthWrite: false,
})));
```

### Turning B−V into a colour

B−V is a temperature measurement, so the conversion goes through the star's
effective temperature. Ballesteros' formula gives it in one step, and a
blackbody approximation turns that into RGB. Stars with no photometry fall back
to white, which is what the eye sees for anything faint anyway.

```js
function colorFromBV(bv) {
  if (bv === null) return { r: 1, g: 1, b: 1 };

  // Ballesteros (2012): B-V to effective temperature in kelvin.
  const t = 4600 * (1 / (0.92 * bv + 1.7) + 1 / (0.92 * bv + 0.62));

  // Compact blackbody approximation, valid over the stellar range.
  const k = Math.min(40000, Math.max(1000, t)) / 100;
  const ch = (v) => Math.min(1, Math.max(0, v / 255));

  const r = k <= 66 ? 255 : 329.7 * Math.pow(k - 60, -0.1332);
  const g = k <= 66 ? 99.47 * Math.log(k) - 161.1
                    : 288.1 * Math.pow(k - 60, -0.0755);
  const b = k >= 66 ? 255
          : k <= 19 ? 0
          : 138.5 * Math.log(k - 10) - 305.0;

  return { r: ch(r), g: ch(g), b: ch(b) };
}
```

Real starlight is far less saturated than this suggests — the eye sees almost
all of these as white. Desaturate toward white if the scene looks like a bag of
sweets; that is a presentation choice, which is exactly why the payload ships
B−V rather than making it for you.

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
jq '.schema, .count, .magnitude_limit' /var/lib/ls-horizons/web/stars.json
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
