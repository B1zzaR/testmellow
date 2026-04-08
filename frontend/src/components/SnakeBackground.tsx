import { useEffect, useRef } from 'react'

// ─── Configuration ────────────────────────────────────────────────────────────

const TICK_MS     = 100   // ms between grid steps  (~6.5 moves / s)
const CELL_LG     = 32     // cell size px — desktop
const CELL_SM     = 22     // cell size px — mobile (viewport < 640 px)
const VIRUS_MAX   = 8      // maximum simultaneous viruses on screen
const INIT_DELAY  = 700    // ms delay before snakes enter
const WANDER_P    = 0.18   // probability of taking a non-optimal direction
const MAX_LEN     = 52     // hard cap on snake body length
const INIT_LEN    = 5      // starting segment count
const GLOW_FRAMES = 22     // render-frames for eat-highlight glow
const DEATH_TICKS     = 8     // game ticks the death-shrink animation lasts


// ─── Types ────────────────────────────────────────────────────────────────────

interface V2 { x: number; y: number }

interface Snake {
  segs:      V2[]    // body segments — index 0 is the head
  dir:       V2      // current heading
  grow:      number  // pending extra segments to add
  flash:     number  // eat-highlight countdown (render frames)
  dead:      boolean // true while playing death animation
  deathTick: number  // ticks remaining in death animation
}

interface Virus {
  pos:    V2
  phase:  number  // oscillation offset for pulse animation
  alpha:  number  // [0–1]; fades in on spawn, out when eaten
  dying:  boolean // true once eaten — triggers fade-out
}

// ─── Pure helpers ─────────────────────────────────────────────────────────────

function cellKey(v: V2)           { return `${v.x},${v.y}` }
function vecEq(a: V2, b: V2)      { return a.x === b.x && a.y === b.y }
function isOpposite(a: V2, b: V2) { return a.x === -b.x && a.y === -b.y }
function manhattan(a: V2, b: V2)  { return Math.abs(a.x - b.x) + Math.abs(a.y - b.y) }

// All four cardinal directions (up / right / down / left)
const DIRS: V2[] = [
  { x:  0, y: -1 },
  { x:  1, y:  0 },
  { x:  0, y:  1 },
  { x: -1, y:  0 },
]

// ─── Component ────────────────────────────────────────────────────────────────

/**
 * SnakeBackground
 *
 * Grid-based classic-Snake animation used as a decorative canvas layer
 * behind the Hero section.  Two snakes wander the grid and actively
 * seek virus particles using greedy Manhattan-distance pathfinding.
 *
 * Rendering: 60 fps via requestAnimationFrame.
 * Game logic: fixed tick rate (TICK_MS) inside the RAF loop.
 *
 * Layering: position:absolute / pointer-events:none / z-index:0 —
 * never blocks any UI element.
 */
export function SnakeBackground() {
  const canvasRef = useRef<HTMLCanvasElement>(null)

  useEffect(() => {
    const canvasEl = canvasRef.current
    if (!canvasEl) return
    const ctxRaw = canvasEl.getContext('2d')
    if (!ctxRaw) return

    // Explicit non-null types so TypeScript preserves them inside closures
    const canvas: HTMLCanvasElement        = canvasEl
    const ctx:    CanvasRenderingContext2D = ctxRaw

    // ── Mutable game state ─────────────────────────────────────────────────

    let alive      = true
    let rafId      = 0
    let lastTickTs = 0
    let W          = 0
    let H          = 0
    let cell       = CELL_LG
    let cols       = 0
    let rows       = 0
    let snakes:  Snake[] = []
    let viruses: Virus[] = []

    // ── Resize ────────────────────────────────────────────────────────────

    function resize() {
      const dpr = Math.min(window.devicePixelRatio || 1, 2)
      W    = canvas.offsetWidth
      H    = canvas.offsetHeight
      cell = W < 640 ? CELL_SM : CELL_LG
      cols = Math.max(1, Math.floor(W / cell))
      rows = Math.max(1, Math.floor(H / cell))
      canvas.width  = W * dpr
      canvas.height = H * dpr
      ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
    }

    // ── Grid helpers ──────────────────────────────────────────────────────

    // Set of all cells currently occupied by snake bodies
    function makeBodySet(): Set<string> {
      const s = new Set<string>()
      for (const sn of snakes) {
        for (const seg of sn.segs) s.add(cellKey(seg))
      }
      return s
    }

    // Pick a random grid cell that is free of snakes and live viruses
    function pickFreeCell(): V2 | null {
      const occ  = makeBodySet()
      const vSet = new Set(viruses.filter(v => !v.dying).map(v => cellKey(v.pos)))
      const m    = 3  // margin from edges
      for (let i = 0; i < 60; i++) {
        const p: V2 = {
          x: m + Math.floor(Math.random() * (cols - m * 2)),
          y: m + Math.floor(Math.random() * (rows - m * 2)),
        }
        if (!occ.has(cellKey(p)) && !vSet.has(cellKey(p))) return p
      }
      return null
    }

    // ── Initialisation ────────────────────────────────────────────────────

    function spawnVirus() {
      if (viruses.filter(v => !v.dying).length >= VIRUS_MAX) return
      const pos = pickFreeCell()
      if (!pos) return
      viruses.push({ pos, phase: Math.random() * Math.PI * 2, alpha: 0, dying: false })
    }

    function initSnakes() {
      if (cols <= 0 || rows <= 0) return
      const midRow = Math.floor(rows / 2)

      // Build a snake with INIT_LEN segments trailing behind the head
      function buildSnake(hx: number, hy: number, dir: V2): Snake {
        const segs: V2[] = []
        for (let i = 0; i < INIT_LEN; i++) {
          segs.push({
            x: Math.max(0, Math.min(cols - 1, hx - dir.x * i)),
            y: Math.max(0, Math.min(rows - 1, hy - dir.y * i)),
          })
        }
        return { segs, dir, grow: 0, flash: 0, dead: false, deathTick: 0 }
      }

      snakes = [
        // Snake 1: enters from the left side, heading right
        buildSnake(1,        Math.max(2, midRow - 4),        { x: 1,  y: 0 }),
        // Snake 2: enters from the right side, heading left
        buildSnake(cols - 2, Math.min(rows - 3, midRow + 4), { x: -1, y: 0 }),
      ]
    }

    // ── Respawn ───────────────────────────────────────────────────────────────

    /**
     * Reset a snake to minimum length entering from a random screen edge.
     * Called after the death animation completes.
     */
    function respawnSnake(sn: Snake) {
      // Pick a random edge: 0=left→right, 1=right→left, 2=top→down, 3=bottom→up
      const edge = Math.floor(Math.random() * 4)
      let hx: number, hy: number, dir: V2

      if (edge === 0) {
        hx = 1;        hy = 2 + Math.floor(Math.random() * Math.max(1, rows - 4)); dir = { x: 1,  y: 0 }
      } else if (edge === 1) {
        hx = cols - 2; hy = 2 + Math.floor(Math.random() * Math.max(1, rows - 4)); dir = { x: -1, y: 0 }
      } else if (edge === 2) {
        hx = 2 + Math.floor(Math.random() * Math.max(1, cols - 4)); hy = 1;        dir = { x: 0,  y: 1 }
      } else {
        hx = 2 + Math.floor(Math.random() * Math.max(1, cols - 4)); hy = rows - 2; dir = { x: 0,  y: -1 }
      }

      const segs: V2[] = []
      for (let i = 0; i < INIT_LEN; i++) {
        segs.push({
          x: Math.max(0, Math.min(cols - 1, hx - dir.x * i)),
          y: Math.max(0, Math.min(rows - 1, hy - dir.y * i)),
        })
      }

      sn.segs      = segs
      sn.dir       = dir
      sn.grow      = 0
      sn.flash     = 0
      sn.dead      = false
      sn.deathTick = 0
    }

    // ── Pathfinding ───────────────────────────────────────────────────────────

    /**
     * Choose the next direction for a snake.
     *
     * Strategy:
     *   1. Build the set of valid directions (not reverse, in-bounds, not self).
     *   2. With probability WANDER_P pick a random valid direction (imperfection).
     *   3. Otherwise: find the nearest live virus, sort valid directions by
     *      Manhattan distance to it, and pick the closest-approach move.
     */
    function chooseDir(sn: Snake): V2 {
      const head = sn.segs[0]

      // Exclude tail tip because it will vacate this tick
      const avoidSet = new Set(sn.segs.slice(0, sn.segs.length - 1).map(cellKey))

      const valid = DIRS.filter(d => {
        if (isOpposite(d, sn.dir)) return false   // no U-turns
        const nx = head.x + d.x
        const ny = head.y + d.y
        if (nx < 0 || nx >= cols || ny < 0 || ny >= rows) return false
        return !avoidSet.has(cellKey({ x: nx, y: ny }))
      })

      // Completely boxed in — allow U-turn as last resort
      if (valid.length === 0) {
        const fallback = DIRS.find(d => {
          const nx = head.x + d.x
          const ny = head.y + d.y
          return nx >= 0 && nx < cols && ny >= 0 && ny < rows
        })
        return fallback ?? sn.dir
      }

      const liveViruses = viruses.filter(v => !v.dying)

      if (liveViruses.length === 0 || Math.random() < WANDER_P) {
        return valid[Math.floor(Math.random() * valid.length)]
      }

      // Nearest virus by Manhattan distance from current head
      const target = liveViruses.reduce((best, v) =>
        manhattan(head, v.pos) < manhattan(head, best.pos) ? v : best
      )

      // Rank valid directions by how much closer they bring us to the target
      const ranked = [...valid].sort((a, b) => {
        const da = manhattan({ x: head.x + a.x, y: head.y + a.y }, target.pos)
        const db = manhattan({ x: head.x + b.x, y: head.y + b.y }, target.pos)
        return da - db
      })

      return ranked[0]
    }

    // ── Game tick (grid logic) ────────────────────────────────────────────────

    function tick() {
      for (const sn of snakes) {
        // ── Death animation: shrink body evenly then respawn ─────────────────
        if (sn.dead) {
          // Remove several tail segments per tick so the body clears within
          // DEATH_TICKS ticks regardless of current length
          const shrink = Math.max(1, Math.ceil(sn.segs.length / sn.deathTick))
          for (let i = 0; i < shrink && sn.segs.length > 0; i++) sn.segs.pop()
          sn.deathTick--
          if (sn.deathTick <= 0 || sn.segs.length === 0) respawnSnake(sn)
          continue
        }

        sn.dir = chooseDir(sn)

        // Compute next head position (clamped to grid)
        const newHead: V2 = {
          x: Math.max(0, Math.min(cols - 1, sn.segs[0].x + sn.dir.x)),
          y: Math.max(0, Math.min(rows - 1, sn.segs[0].y + sn.dir.y)),
        }

        // ── Collision detection (checked BEFORE moving) ───────────────────────
        // Self-collision: exclude tail tip because it will vacate this tick
        const selfHit = sn.segs
          .slice(0, sn.segs.length - 1)
          .some(s => vecEq(s, newHead))

        // Other-snake collision: any segment of any other live snake
        const otherHit = snakes.some(
          other => other !== sn && !other.dead &&
          other.segs.some(s => vecEq(s, newHead))
        )

        if (selfHit || otherHit) {
          // Start death animation — snake will shrink and then re-enter
          sn.dead      = true
          sn.deathTick = DEATH_TICKS
          sn.grow      = 0
          sn.flash     = 0
          continue
        }

        // ── Normal movement ───────────────────────────────────────────────────
        sn.segs.unshift(newHead)

        // Consume pending growth or remove the tail
        if (sn.grow > 0) {
          sn.grow--
        } else {
          sn.segs.pop()
        }

        // Hard length cap
        if (sn.segs.length > MAX_LEN) sn.segs.length = MAX_LEN

        // Virus collision — eat on cell overlap
        for (const v of viruses) {
          if (!v.dying && vecEq(newHead, v.pos)) {
            v.dying  = true
            sn.grow += 1
            sn.flash = GLOW_FRAMES
            setTimeout(spawnVirus, 500)
          }
        }
      }

      // Purge fully invisible dead viruses
      viruses = viruses.filter(v => !(v.dying && v.alpha < 0.005))
    }

    // ── Drawing ───────────────────────────────────────────────────────────────

    function drawSnake(sn: Snake) {
      const len = sn.segs.length
      if (len === 0) return

      // ── Death animation: red/orange segments shrinking from the tail ─────────
      if (sn.dead) {
        const deathT = sn.deathTick / DEATH_TICKS  // 1→0 as death progresses
        for (let i = len - 1; i >= 0; i--) {
          const segT  = i / Math.max(len - 1, 1)
          const alpha = deathT * (1 - segT * 0.4)
          const pad   = 3
          const s     = cell - pad * 2
          if (i === 0) {
            ctx.shadowColor = `rgba(255,80,30,${alpha * 0.9})`
            ctx.shadowBlur  = 18
          } else if (i === 1) {
            ctx.shadowBlur  = 0
            ctx.shadowColor = 'transparent'
          }
          ctx.beginPath()
          ctx.roundRect(sn.segs[i].x * cell + pad, sn.segs[i].y * cell + pad, s, s, 5)
          ctx.fillStyle = `rgba(255,100,40,${alpha})`
          ctx.fill()
        }
        ctx.shadowBlur  = 0
        ctx.shadowColor = 'transparent'
        return
      }

      const fl = sn.flash > 0

      // Decrement flash counter once per rendered frame
      if (sn.flash > 0) sn.flash--

      // Render segments tail → head so the head always draws on top
      for (let i = len - 1; i >= 0; i--) {
        const t     = i / Math.max(len - 1, 1)       // 0 = head, 1 = tail tip
        const alpha = (fl ? 0.95 : 0.82) * (1 - t * 0.62)
        const pad   = 3
        const s     = cell - pad * 2

        // Glow only on the head; cleared immediately after to avoid bleed
        if (i === 0) {
          ctx.shadowColor = fl ? 'rgba(134,239,172,0.9)' : 'rgba(74,222,128,0.6)'
          ctx.shadowBlur  = fl ? 22 : 14
        } else if (i === 1) {
          ctx.shadowBlur  = 0
          ctx.shadowColor = 'transparent'
        }

        ctx.beginPath()
        ctx.roundRect(
          sn.segs[i].x * cell + pad,
          sn.segs[i].y * cell + pad,
          s, s, 5,
        )
        ctx.fillStyle = fl
          ? `rgba(187,247,208,${alpha})`
          : `rgba(74,222,128,${alpha})`
        ctx.fill()
      }

      ctx.shadowBlur  = 0
      ctx.shadowColor = 'transparent'
    }

    function drawVirus(v: Virus) {
      if (v.alpha < 0.005) return

      // Pulsing scale animation
      const pulse  = 1 + 0.08 * Math.sin(v.phase)
      const cx     = v.pos.x * cell + cell * 0.5
      const cy     = v.pos.y * cell + cell * 0.5
      const r      = cell * 0.18 * pulse   // body radius
      const spiLen = cell * 0.14           // spike length
      const spiCnt = 6                     // spike count — minimal

      // Subtle green glow
      ctx.shadowColor = `rgba(74,222,128,${v.alpha * 0.4})`
      ctx.shadowBlur  = 10

      // Body — thin outlined circle only (minimalist)
      ctx.beginPath()
      ctx.arc(cx, cy, r, 0, Math.PI * 2)
      ctx.strokeStyle = `rgba(74,222,128,${v.alpha * 0.75})`
      ctx.lineWidth   = 1.5
      ctx.stroke()

      // Tiny filled center dot
      ctx.shadowBlur = 0
      ctx.beginPath()
      ctx.arc(cx, cy, r * 0.28, 0, Math.PI * 2)
      ctx.fillStyle = `rgba(134,239,172,${v.alpha * 0.8})`
      ctx.fill()

      // 6 minimal spikes that slowly rotate
      ctx.strokeStyle = `rgba(74,222,128,${v.alpha * 0.6})`
      ctx.lineWidth   = 1.2
      ctx.lineCap     = 'round'
      for (let i = 0; i < spiCnt; i++) {
        const angle = (i / spiCnt) * Math.PI * 2 + v.phase * 0.10
        const x1 = cx + Math.cos(angle) * (r + 1)
        const y1 = cy + Math.sin(angle) * (r + 1)
        const x2 = cx + Math.cos(angle) * (r + spiLen)
        const y2 = cy + Math.sin(angle) * (r + spiLen)
        ctx.beginPath()
        ctx.moveTo(x1, y1)
        ctx.lineTo(x2, y2)
        ctx.stroke()
        // Tip dot
        ctx.beginPath()
        ctx.arc(x2, y2, 1.5, 0, Math.PI * 2)
        ctx.fillStyle = `rgba(134,239,172,${v.alpha * 0.7})`
        ctx.fill()
      }

      ctx.lineCap     = 'butt'
      ctx.shadowColor = 'transparent'
      ctx.shadowBlur  = 0
    }

    // ── Render frame ──────────────────────────────────────────────────────────

    function render() {
      ctx.clearRect(0, 0, W, H)

      // Animate virus opacity and oscillation (runs every frame)
      for (const v of viruses) {
        v.phase += 0.035
        v.alpha  = v.dying
          ? Math.max(0, v.alpha - 0.055)
          : Math.min(0.88, v.alpha + 0.018)
      }

      // Viruses drawn first — they appear behind snakes
      for (const v of viruses)  drawVirus(v)
      for (const sn of snakes)  drawSnake(sn)
    }

    // ── Main loop ─────────────────────────────────────────────────────────────

    function loop(now: DOMHighResTimeStamp) {
      if (!alive) return

      // Advance game state at a fixed interval (decoupled from frame rate)
      if (now - lastTickTs >= TICK_MS) {
        lastTickTs = now
        tick()
      }

      render()
      rafId = requestAnimationFrame(loop)
    }

    // ── Bootstrap ─────────────────────────────────────────────────────────────

    resize()

    // Pre-populate the board with viruses so they're visible before snakes enter
    for (let i = 0; i < VIRUS_MAX; i++) spawnVirus()

    // Snakes enter after the initial delay
    const initTimer = setTimeout(() => {
      if (!alive) return
      initSnakes()
      lastTickTs = performance.now()
    }, INIT_DELAY)

    rafId = requestAnimationFrame(loop)

    // Re-measure on container resize
    const ro = new ResizeObserver(() => {
      if (!alive) return
      resize()
    })
    ro.observe(canvas)

    return () => {
      alive = false
      clearTimeout(initTimer)
      cancelAnimationFrame(rafId)
      ro.disconnect()
    }
  }, [])

  return (
    <canvas
      ref={canvasRef}
      className="pointer-events-none absolute inset-0 h-full w-full"
      aria-hidden="true"
      style={{ zIndex: 0 }}
    />
  )
}

