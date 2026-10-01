/*
 * g360-ventas-api.build.cjs - documentacion del API g360-ventas-api.
 *
 * Compone el deck como codigo contra los tokens de la marca (paleta corporativa
 * G360: tinta #0b1220, papel #f0f4f8, acento teal #00796B, muted #94a3b8).
 *
 * Decisiones del deck (fijadas antes de dibujar):
 *   Suelo: sandwich. Tapa (s1) y cierre (s10) sobre tinta; la evidencia (s2-s9)
 *          sobre papel. El deck abre, trabaja y aterriza.
 *   Dominancia: papel y tinta llevan el 90% del peso. El acento teal se gasta
 *          solo en el elemento que gana cada slide: el panel del API (s3), la
 *          tarjeta heroe (s5), el stat 201/201 (s6), el panel ganador (s7) y el
 *          numero del turno (s9). Nunca mas de dos marcas por slide.
 *   Motivo: la cadena de tres nodos (productor, API, cliente) unida por flechas
 *          finas. Aparece como diagrama en s3, como eco del camino manual en s2,
 *          y como marca fantasma en la tapa y el cierre.
 *   Escalera de tipos: cinco pasos de T.scale (72/48/32/18/12). El tamano
 *          jerarquiza; el color nunca solo.
 *   Contraste: sobre papel el muted da 2.51 y falla, asi que el texto pequeno va
 *          en ink; sobre tinta el muted da 6.77 y el accent 3.52 (solo texto
 *          grande). Por eso el acento nunca sostiene texto pequeno.
 *
 * Ejecutar:
 *   node g360-ventas-api.build.cjs --tokens .slides/tokens.cjs --out g360-ventas-api.pptx
 *
 * Reglas que sigue este script:
 *   - El contenido es dato: cada palabra vive en el bloque JSON C.
 *   - Solo tokens: cada color, fuente y tamano sale de T.
 *   - Stamps: objectName dice de que campo del spec salio cada texto
 *     (slides-field:*, un slides-lead por slide, slides-ghost para la marca
 *     fantasma, deco-* para lo dibujado).
 *   - Las notas viajan textuales (addNotes) para que el deck vuelva al spec.
 */

const argv = process.argv.slice(2);

function arg(name) {
  const i = argv.indexOf(name);
  return i >= 0 && i + 1 < argv.length ? argv[i + 1] : null;
}

const tokensPath = arg("--tokens");
const outPath = arg("--out");

if (!tokensPath || !outPath) {
  console.error("usage: node g360-ventas-api.build.cjs --tokens <tokens.cjs> --out <deck.pptx>");
  process.exit(2);
}

const T = require(require("path").resolve(tokensPath));
const pptxgen = T.loadPptxgen();

// Cada palabra de cada slide, en un solo lugar.
const C = JSON.parse('{"s1":{"title":"g360-ventas-api","subtitle":"La base canónica de ventas, en solo lectura por HTTP a la intranet","notes":"Contexto para el lector: el productor de la base canónica es la app Tauri de ventas y esta API la sirve por HTTP. El deck documenta qué expone, cómo se verificó y cómo se despliega."},"s2":{"title":"Hoy el cliente depende de copias manuales","body":"db_network.py lee la BD completa por SMB o por USB en zip: 2,8M de filas por pasada, sin control de frescura y con el archivo productor expuesto al acceso por red.","labels":["ERP cliente","SMB o USB","copia de historial.db"],"notes":"El problema actual del cliente ERP: cada sincronización lee todo y el equipo no sabe qué cambió desde la última pasada. La API resuelve las dos cosas con checksums y folios."},"s3":{"title":"La cadena, de extremo a extremo","nodes":[{"h":"Tauri (Rust)","cap":"escritor único de historial.db"},{"h":"API Go","cap":"read-only :8090"},{"h":"Cliente ERP","cap":"diff de folios y checksums"}],"notes":"Un solo escritor (Tauri), una API de solo lectura, y los clientes que consultan por HTTP. La BD nunca se comparte por red como archivo."},"s4":{"title":"Sync incremental en cinco pasos","steps":[{"n":"1","label":"Login","detail":"credenciales intranet, token HMAC de 24 h"},{"n":"2","label":"Status y checksums","detail":"qué meses cambiaron desde la última sync"},{"n":"3","label":"Folios","detail":"folios del rango, consulta liviana"},{"n":"4","label":"By-folios","detail":"filas completas de los faltantes, máx 500 por request"},{"n":"5","label":"Insert local","detail":"dedup por folio con la lógica existente del cliente"}],"notes":"El cliente ya conoce este flujo: es el mismo contrato de contraster y traer folios faltantes que hoy hace por SMB, ahora por HTTP con token."},"s5":{"title":"13 endpoints, tres grupos","hero":{"label":"Sync incremental","body":"6 endpoints: status, checksums, day-checksums, folios, by-folios, contrast"},"cards":[{"label":"Auth","body":"2 endpoints: login, health"},{"label":"Modelo completo","body":"5 endpoints: model, data, query, stats, export"}],"notes":"El grupo de sync alimenta al cliente en producción. El modelo completo permite trabajar offline con allowlist. La base canónica se descarga por export."},"s6":{"title":"Verificado contra producción","stats":[{"v":"2,8M","label":"filas servidas"},{"v":"13/13","label":"tests de humo"},{"v":"201/201","label":"checksums vs productor","lead":true},{"v":"8s","label":"status en frío"}],"notes":"Verificación del 1 de octubre de 2026 contra la BD real: smoke test de 13 endpoints, paridad del checksum live contra la tabla mes_checksums del productor, y status en 8 segundos sobre el snapshot en ext4."},"s7":{"title":"Despliegue en esta máquina","winner":{"label":"WSL2 + forwarder","body":"funciona: API Go en Ubuntu, snapshot en ext4 y forwarder Python en el 8090, sin admin"},"loser":{"label":"Windows nativo","body":"bloqueado: FortiEDR impide el listen() de binarios Go sin cadena de firma confiable"},"notes":"FortiEDR bloquea el listen() de binarios Go no firmados en Windows. Por eso el API corre en WSL2 y un forwarder Python expone el puerto a la intranet. La alternativa con admin es netsh portproxy más regla de firewall."},"s8":{"title":"Seguridad por diseño","body":"La conexión SQLite abre en mode=ro con query_only: escribir es imposible. La whitelist expone 24 objetos y deja fuera audit_log, sync_log y sync_months. El login compara credenciales en constant-time y emite un token HMAC con expiración.","layers":[{"label":"mode=ro + query_only","cap":"la conexión no puede escribir"},{"label":"whitelist de 24 objetos","cap":"audit_log y sync_log quedan fuera"},{"label":"token HMAC con expiración","cap":"credenciales intranet en constant-time"}],"notes":"Tres capas: la conexión no puede escribir, la whitelist limita qué se ve, y el token limita quién entra. Los valores van siempre parametrizados y los identificadores se validan contra la whitelist."},"s9":{"title":"Fase 2: el cliente deja el SMB","steps":[{"n":"1","label":"Adaptador Python","detail":"reemplaza db_network.py: login, checksums, folios, by-folios"},{"n":"2","label":"Validación","detail":"diff contra SMB en un mes real de datos","turn":true},{"n":"3","label":"Producción","detail":"despliegue en la intranet y excepción FortiEDR si se corre nativo"}],"notes":"Fase 1 completa: API verificada y desplegada. Fase 2 reemplaza el transporte del cliente; el contrato de sync ya está probado, así que el adaptador solo cambia el transporte."},"s10":{"statement":"Una sola base canónica, servida por HTTP a toda la intranet.","notes":"Cierre: el archivo productor deja de circular como copia; los clientes se ponen al día por HTTP contra una sola fuente. El siguiente paso es el adaptador del cliente (fase 2)."}}');

const pres = new pptxgen();
pres.layout = "LAYOUT_WIDE";

const W = T.slide.w;
const H = T.slide.h;
const MX = T.margins.x;
const MT = T.margins.top;
const CONTENT_W = W - 2 * MX;
const HAIR = T.shape.hairline_pt || 1;
const ROUNDED = T.shape.corner === "rounded";

// Fresh options object per call; margin 0 so text aligns with the shapes.
function text(slide, body, opts) {
  slide.addText(body, Object.assign({ margin: 0 }, opts));
}

function mark(slide, kind, opts) {
  slide.addShape(kind, Object.assign({}, opts));
}

function panel(slide, opts) {
  const o = Object.assign({}, opts);
  if (ROUNDED) {
    o.rectRadius = 0.12;
    slide.addShape(pres.shapes.ROUNDED_RECTANGLE, o);
  } else {
    slide.addShape(pres.shapes.RECTANGLE, o);
  }
}

// A grey that stays on-brand: a token colour, thinned.
function tint(colour, transparency) {
  return { color: colour, transparency: transparency };
}

// The motif: three nodes joined by thin arrows. Drawn as the subject on the
// architecture slide, as the echo of the manual path on the problem slide, and
// as a single deliberate ghost mark on the bookends.
function chain(slide, x, y, node, gap, opts) {
  const o = opts || {};
  const colour = o.colour || T.colours.muted;
  const trans = o.transparency == null ? 72 : o.transparency;
  const names = o.objectPrefix || "deco-chain";
  for (let i = 0; i < 2; i++) {
    mark(slide, pres.shapes.LINE, {
      x: x + node + i * (node + gap), y: y + node / 2, w: gap, h: 0,
      line: { color: colour, width: 1.5, transparency: trans, endArrowType: "triangle" },
      objectName: names + "-link-" + (i + 1),
    });
  }
  for (let i = 0; i < 3; i++) {
    mark(slide, pres.shapes.RECTANGLE, {
      x: x + i * (node + gap), y: y, w: node, h: node,
      fill: tint(colour, trans), line: tint(colour, trans),
      objectName: names + "-node-" + (i + 1),
    });
  }
}

// ---------------------------------------------------------------- 1. title --
{
  const s = pres.addSlide();
  s.background = { color: T.colours.ink };

  // The motif as a ghost mark, lower right, well clear of the hierarchy.
  chain(s, 10.55, 6.32, 0.28, 0.42, { objectPrefix: "slides-ghost" });

  text(s, C.s1.title, {
    x: MX, y: 2.6, w: 11.0, h: 1.6,
    fontFace: T.fonts.heading, fontSize: T.scale.title, bold: true,
    color: T.colours.paper, valign: "bottom", lineSpacingMultiple: 0.95,
    objectName: "slides-lead:Title",
  });
  text(s, C.s1.subtitle, {
    x: MX, y: 4.4, w: 10.5, h: 0.6,
    fontFace: T.fonts.body, fontSize: T.scale.body, color: T.colours.muted,
    objectName: "slides-field:Subtitle",
  });

  s.addNotes(C.s1.notes);
}

// --------------------------------------------------------------- 2. problem --
// The manual path drawn small: where the file goes today.
{
  const s = pres.addSlide();
  s.background = { color: T.colours.paper };

  text(s, C.s2.title, {
    x: MX, y: MT + 0.05, w: 11.4, h: 1.0,
    fontFace: T.fonts.heading, fontSize: T.scale.h1, bold: true,
    color: T.colours.ink, valign: "top", objectName: "slides-lead:Title",
  });
  text(s, C.s2.body, {
    x: MX, y: 1.8, w: 10.6, h: 1.2,
    fontFace: T.fonts.body, fontSize: T.scale.body, color: T.colours.ink,
    valign: "top", objectName: "slides-field:Body",
  });

  // the path today, drawn as the same three-node chain the real one takes
  const nn = 1.2;
  const ng = 1.3;
  const n0 = (W - (3 * nn + 2 * ng)) / 2;
  for (let i = 0; i < 2; i++) {
    mark(s, pres.shapes.LINE, {
      x: n0 + (i + 1) * nn + i * ng, y: 4.1, w: ng, h: 0,
      line: { color: T.colours.ink, width: 2, endArrowType: "triangle" },
      objectName: "deco-manual-link-" + (i + 1),
    });
  }
  for (let i = 0; i < 3; i++) {
    const x = n0 + i * (nn + ng);
    panel(s, {
      x: x, y: 3.5, w: nn, h: nn,
      fill: tint(T.colours.muted, 78), line: { color: T.colours.ink, width: HAIR },
      objectName: "deco-manual-node-" + (i + 1),
    });
    text(s, C.s2.labels[i], {
      x: x - 0.5, y: 4.9, w: nn + 1.0, h: 0.4,
      fontFace: T.fonts.body, fontSize: T.scale.caption, color: T.colours.ink,
      align: "center", objectName: "deco-manual-label-" + (i + 1),
    });
  }

  s.addNotes(C.s2.notes);
}

// ------------------------------------------------------------ 3. chain map --
{
  const s = pres.addSlide();
  s.background = { color: T.colours.paper };

  text(s, C.s3.title, {
    x: MX, y: MT + 0.05, w: 11.4, h: 0.9,
    fontFace: T.fonts.heading, fontSize: T.scale.h1, bold: true,
    color: T.colours.ink, valign: "top", objectName: "slides-lead:Title",
  });

  const py = 2.35;
  const ph = 2.2;
  const pw = 3.49;
  const xs = [0.667, 4.917, 9.167];
  for (let i = 0; i < 2; i++) {
    mark(s, pres.shapes.LINE, {
      x: xs[i] + pw, y: 3.45, w: 0.75, h: 0,
      line: { color: T.colours.ink, width: 2, endArrowType: "triangle" },
      objectName: "deco-chain-arrow-" + (i + 1),
    });
  }
  for (let i = 0; i < 3; i++) {
    const hero = i === 1;
    panel(s, {
      x: xs[i], y: py, w: pw, h: ph,
      fill: hero ? { color: T.colours.accent } : tint(T.colours.muted, 78),
      line: { color: hero ? T.colours.accent : T.colours.ink, width: HAIR },
      objectName: "deco-chain-box-" + (i + 1),
    });
    const fg = hero ? T.colours.paper : T.colours.ink;
    // node headings sit at body size: the slide title (h1) is the lead, so the
    // panels support it rather than compete with it
    text(s, C.s3.nodes[i].h, {
      x: xs[i] + 0.25, y: py + 0.35, w: pw - 0.5, h: 0.6,
      fontFace: T.fonts.heading, fontSize: T.scale.body, bold: true,
      color: fg, valign: "top", objectName: "slides-field:Block",
    });
    text(s, C.s3.nodes[i].cap, {
      x: xs[i] + 0.25, y: py + 1.05, w: pw - 0.5, h: 0.8,
      fontFace: T.fonts.body, fontSize: T.scale.caption, color: fg,
      valign: "top", objectName: "slides-field:Block",
    });
  }

  s.addNotes(C.s3.notes);
}

// ------------------------------------------------------------- 4. the flow --
{
  const s = pres.addSlide();
  s.background = { color: T.colours.paper };

  text(s, C.s4.title, {
    x: MX, y: MT + 0.05, w: 11.4, h: 0.9,
    fontFace: T.fonts.heading, fontSize: T.scale.h1, bold: true,
    color: T.colours.ink, valign: "top", objectName: "slides-lead:Title",
  });

  const n = C.s4.steps.length;
  const bw = 2.04;
  const step = (CONTENT_W - bw) / (n - 1);
  for (let i = 0; i < n; i++) {
    const bx = MX + i * step;
    panel(s, {
      x: bx, y: 2.5, w: bw, h: 2.5,
      fill: tint(T.colours.muted, 82), line: { color: T.colours.ink, width: HAIR },
      objectName: "deco-step-box-" + (i + 1),
    });
    if (i < n - 1) {
      mark(s, pres.shapes.LINE, {
        x: bx + bw, y: 3.75, w: step - bw, h: 0,
        line: { color: T.colours.ink, width: 2, endArrowType: "triangle" },
        objectName: "deco-step-arrow-" + (i + 1),
      });
    }
    text(s, [
      { text: C.s4.steps[i].n + " ", options: { color: T.colours.accent, bold: true } },
      { text: C.s4.steps[i].label, options: { color: T.colours.ink, bold: true } },
    ], {
      x: bx + 0.18, y: 2.75, w: bw - 0.36, h: 1.0,
      fontFace: T.fonts.body, fontSize: T.scale.body,
      valign: "top", objectName: "slides-field:Block",
    });
    text(s, C.s4.steps[i].detail, {
      x: bx + 0.18, y: 3.85, w: bw - 0.36, h: 1.0,
      fontFace: T.fonts.body, fontSize: T.scale.caption, color: T.colours.ink,
      valign: "top", objectName: "slides-field:Block",
    });
  }

  s.addNotes(C.s4.notes);
}

// ----------------------------------------------------------- 5. endpoints --
{
  const s = pres.addSlide();
  s.background = { color: T.colours.paper };

  text(s, C.s5.title, {
    x: MX, y: MT + 0.05, w: 11.4, h: 0.9,
    fontFace: T.fonts.heading, fontSize: T.scale.h1, bold: true,
    color: T.colours.ink, valign: "top", objectName: "slides-lead:Title",
  });

  panel(s, {
    x: 0.667, y: 2.4, w: 5.6, h: 2.7,
    fill: { color: T.colours.accent }, line: { color: T.colours.accent },
    objectName: "deco-card-hero",
  });
  text(s, C.s5.hero.label, {
    x: 1.017, y: 2.75, w: 4.9, h: 0.6,
    fontFace: T.fonts.body, fontSize: T.scale.body, bold: true,
    color: T.colours.paper, objectName: "slides-field:Block",
  });
  text(s, C.s5.hero.body, {
    x: 1.017, y: 3.6, w: 4.9, h: 1.3,
    fontFace: T.fonts.body, fontSize: T.scale.body, color: T.colours.paper,
    valign: "top", objectName: "slides-field:Block",
  });

  for (let i = 0; i < C.s5.cards.length; i++) {
    const cx = 6.87 + i * 3.1;
    panel(s, {
      x: cx, y: 2.4, w: 2.69, h: 2.7,
      fill: tint(T.colours.muted, 82), line: { color: T.colours.ink, width: HAIR },
      objectName: "deco-card-" + (i + 1),
    });
    text(s, C.s5.cards[i].label, {
      x: cx + 0.25, y: 2.75, w: 2.2, h: 0.9,
      fontFace: T.fonts.body, fontSize: T.scale.body, bold: true,
      color: T.colours.ink, valign: "top", objectName: "slides-field:Block",
    });
    text(s, C.s5.cards[i].body, {
      x: cx + 0.25, y: 3.75, w: 2.2, h: 1.1,
      fontFace: T.fonts.body, fontSize: T.scale.caption, color: T.colours.ink,
      valign: "top", objectName: "slides-field:Block",
    });
  }

  s.addNotes(C.s5.notes);
}

// ---------------------------------------------------------- 6. proof stat --
{
  const s = pres.addSlide();
  s.background = { color: T.colours.paper };

  text(s, C.s6.title, {
    x: MX, y: MT + 0.05, w: 11.4, h: 0.9,
    fontFace: T.fonts.heading, fontSize: T.scale.h1, bold: true,
    color: T.colours.ink, valign: "top", objectName: "slides-field:Title",
  });

  mark(s, pres.shapes.LINE, {
    x: MX, y: 2.9, w: CONTENT_W, h: 0,
    line: { color: T.colours.ink, width: 2, transparency: 60 },
    objectName: "deco-rule-stats",
  });

  const colW = (CONTENT_W - 0.6) / 4;
  for (let i = 0; i < C.s6.stats.length; i++) {
    const st = C.s6.stats[i];
    const x = MX + i * (colW + 0.2);
    // the lead stat takes the title step; the others step down to h1 so the
    // hero number leads and the rest support it
    text(s, st.v, {
      x: x, y: 3.2, w: colW, h: 1.1,
      fontFace: T.fonts.heading, fontSize: st.lead ? T.scale.title : T.scale.h1, bold: true,
      color: st.lead ? T.colours.accent : T.colours.ink, valign: "middle",
      objectName: st.lead ? "slides-lead" : "slides-field:Block",
    });
    text(s, st.label, {
      x: x, y: 4.45, w: colW, h: 0.6,
      fontFace: T.fonts.body, fontSize: T.scale.caption, color: T.colours.ink,
      objectName: "slides-field:Block",
    });
  }

  s.addNotes(C.s6.notes);
}

// ------------------------------------------------------- 7. deploy verdict --
{
  const s = pres.addSlide();
  s.background = { color: T.colours.paper };

  text(s, C.s7.title, {
    x: MX, y: MT + 0.05, w: 11.4, h: 0.9,
    fontFace: T.fonts.heading, fontSize: T.scale.h1, bold: true,
    color: T.colours.ink, valign: "top", objectName: "slides-lead:Title",
  });

  panel(s, {
    x: MX, y: 2.4, w: 5.69, h: 3.3,
    fill: { color: T.colours.accent }, line: { color: T.colours.accent },
    objectName: "deco-verdict-winner",
  });
  text(s, C.s7.winner.label, {
    x: 1.017, y: 2.8, w: 5.0, h: 0.6,
    fontFace: T.fonts.body, fontSize: T.scale.body, bold: true,
    color: T.colours.paper, objectName: "slides-field:Block",
  });
  text(s, C.s7.winner.body, {
    x: 1.017, y: 3.65, w: 5.0, h: 1.9,
    fontFace: T.fonts.body, fontSize: T.scale.body, color: T.colours.paper,
    valign: "top", objectName: "slides-field:Block",
  });

  panel(s, {
    x: 6.97, y: 2.4, w: 5.69, h: 3.3,
    fill: tint(T.colours.muted, 82), line: { color: T.colours.ink, width: HAIR },
    objectName: "deco-verdict-loser",
  });
  text(s, C.s7.loser.label, {
    x: 7.32, y: 2.8, w: 5.0, h: 0.6,
    fontFace: T.fonts.body, fontSize: T.scale.body, bold: true,
    color: T.colours.ink, objectName: "slides-field:Block",
  });
  text(s, C.s7.loser.body, {
    x: 7.32, y: 3.65, w: 5.0, h: 1.9,
    fontFace: T.fonts.body, fontSize: T.scale.body, color: T.colours.ink,
    valign: "top", objectName: "slides-field:Block",
  });

  s.addNotes(C.s7.notes);
}

// ------------------------------------------------------------ 8. security --
{
  const s = pres.addSlide();
  s.background = { color: T.colours.paper };

  text(s, C.s8.title, {
    x: MX, y: MT + 0.05, w: 11.4, h: 0.9,
    fontFace: T.fonts.heading, fontSize: T.scale.h1, bold: true,
    color: T.colours.ink, valign: "top", objectName: "slides-lead:Title",
  });
  text(s, C.s8.body, {
    x: MX, y: 1.8, w: 11.0, h: 1.3,
    fontFace: T.fonts.body, fontSize: T.scale.body, color: T.colours.ink,
    valign: "top", objectName: "slides-field:Body",
  });

  for (let i = 0; i < C.s8.layers.length; i++) {
    const by = 3.5 + i * 1.05;
    panel(s, {
      x: MX, y: by, w: 9.5, h: 0.85,
      fill: tint(T.colours.muted, 82), line: { color: T.colours.ink, width: HAIR },
      objectName: "deco-layer-box-" + (i + 1),
    });
    text(s, C.s8.layers[i].label, {
      x: MX + 0.3, y: by + 0.18, w: 4.6, h: 0.5,
      fontFace: T.fonts.body, fontSize: T.scale.body, bold: true,
      color: T.colours.ink, valign: "middle", objectName: "slides-field:Block",
    });
    text(s, C.s8.layers[i].cap, {
      x: MX + 5.1, y: by + 0.28, w: 4.1, h: 0.35,
      fontFace: T.fonts.body, fontSize: T.scale.caption, color: T.colours.ink,
      align: "right", valign: "middle", objectName: "slides-field:Block",
    });
  }

  s.addNotes(C.s8.notes);
}

// ------------------------------------------------------------- 9. fase dos --
{
  const s = pres.addSlide();
  s.background = { color: T.colours.paper };

  text(s, C.s9.title, {
    x: MX, y: MT + 0.05, w: 11.4, h: 0.9,
    fontFace: T.fonts.heading, fontSize: T.scale.h1, bold: true,
    color: T.colours.ink, valign: "top", objectName: "slides-lead:Title",
  });

  const n = C.s9.steps.length;
  const pw = 3.49;
  const xs = [0.667, 4.917, 9.167];
  for (let i = 0; i < n; i++) {
    if (i < n - 1) {
      mark(s, pres.shapes.LINE, {
        x: xs[i] + pw, y: 3.7, w: 0.75, h: 0,
        line: { color: T.colours.ink, width: 2, endArrowType: "triangle" },
        objectName: "deco-phase-arrow-" + (i + 1),
      });
    }
    panel(s, {
      x: xs[i], y: 2.5, w: pw, h: 2.4,
      fill: tint(T.colours.muted, 82), line: { color: T.colours.ink, width: HAIR },
      objectName: "deco-phase-box-" + (i + 1),
    });
    text(s, [
      { text: C.s9.steps[i].n + " ", options: { color: C.s9.steps[i].turn ? T.colours.accent : T.colours.ink, bold: true } },
      { text: C.s9.steps[i].label, options: { color: T.colours.ink, bold: true } },
    ], {
      x: xs[i] + 0.25, y: 2.75, w: pw - 0.5, h: 0.9,
      fontFace: T.fonts.body, fontSize: T.scale.body,
      valign: "top", objectName: "slides-field:Block",
    });
    text(s, C.s9.steps[i].detail, {
      x: xs[i] + 0.25, y: 3.75, w: pw - 0.5, h: 0.9,
      fontFace: T.fonts.body, fontSize: T.scale.caption, color: T.colours.ink,
      valign: "top", objectName: "slides-field:Block",
    });
  }

  s.addNotes(C.s9.notes);
}

// -------------------------------------------------------------- 10. close --
{
  const s = pres.addSlide();
  s.background = { color: T.colours.ink };

  text(s, C.s10.statement, {
    x: MX, y: 2.7, w: 11.4, h: 2.2,
    fontFace: T.fonts.heading, fontSize: T.scale.title, bold: true,
    color: T.colours.paper, valign: "top", lineSpacingMultiple: 1.0,
    objectName: "slides-lead:Statement",
  });

  chain(s, 10.55, 6.32, 0.28, 0.42, { objectPrefix: "slides-ghost" });

  s.addNotes(C.s10.notes);
}

pres
  .writeFile({ fileName: outPath })
  .then(() => console.log("wrote " + outPath))
  .catch((err) => {
    console.error(err.message);
    process.exit(1);
  });
