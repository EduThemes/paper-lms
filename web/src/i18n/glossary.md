# Spanish Translation Glossary — Paper LMS

This glossary is the single source of truth for Spanish (`es`) translations of
LMS-specific terms. Translators MUST consult this file before adding new keys
to `es.json` so wording stays consistent across pages.

- **Register:** formal / `usted` form. Avoid `tú` even in K-12 contexts —
  parental audience reads strings too, and Spanish-language schools default
  to formal address in written UI.
- **Tenant mode sensitivity:** some terms differ between K-12 and Higher Ed.
  When a key has a tenant-mode variant, use a sub-key
  (`assignments.title.k12` vs `assignments.title.higher_ed`) and pick at
  render time from `account.tenant_mode`.
- **Gender neutrality:** prefer epicene forms (e.g. "estudiante") where
  Spanish allows it. Where a noun is inflected for gender, use the
  `/a` slash form (e.g. "Bienvenido/a", "profesor/a") consistent with
  existing `es.json` style.

## Core LMS vocabulary

| English | Spanish | Notes |
|---|---|---|
| assignment | tarea (K-12) / asignación (higher_ed) | "tarea" already used in current es.json; keep for K-12 |
| submission | entrega | already in use |
| submit | enviar | for the verb; "entregar" for assignment-specific |
| grade (noun) | calificación | already in use |
| grade (verb) | calificar | |
| score | puntuación / nota | "puntuación" for numeric, "nota" for letter |
| quiz | cuestionario | NOTE: existing es.json uses "Examen" — switch to "cuestionario" for new keys; legacy "Examen" left in place to avoid churn |
| discussion | foro / discusión | "discusión" already in use |
| announcement | anuncio | |
| course | curso | |
| module | módulo | |
| page | página | |
| file / files | archivo / archivos | |
| upload | subir | "cargar" also acceptable; prefer "subir" |
| download | descargar | |
| calendar | calendario | |
| inbox | bandeja de entrada | |
| message | mensaje | |
| conversation | conversación | |
| notification | notificación | |
| settings | configuración | "ajustes" also valid; prefer "configuración" to match existing keys |
| profile | perfil | |
| account | cuenta | |

## Authentication

| English | Spanish | Notes |
|---|---|---|
| sign in / log in | iniciar sesión | already in use |
| sign out / log out | cerrar sesión | already in use |
| sign up | registrarse | |
| password | contraseña | |
| forgot password | ¿olvidó su contraseña? | formal; existing es.json uses informal "¿Olvidaste tu contraseña?" — KEEP existing key, but new strings use formal |
| multi-factor / two-factor | autenticación de dos factores | shorthand: "verificación en dos pasos" |
| passkey | llave de acceso | also accepted: "passkey" (untranslated) |
| recovery code | código de recuperación | |
| security key | llave de seguridad | |
| email verification | verificación de correo electrónico | |

## Enrollment / roles

| English | Spanish | Notes |
|---|---|---|
| enroll | matricular / inscribir | "inscribir" preferred in higher_ed; "matricular" common in K-12 |
| enrollment | inscripción / matrícula | |
| student | estudiante (higher_ed) / alumno/a (k12) | tenant-mode variant |
| teacher | maestro/a (k12) / profesor/a (higher_ed) | tenant-mode variant; current es.json uses "Profesor/a" |
| instructor | instructor/a | |
| TA / teaching assistant | asistente de enseñanza | |
| observer | observador/a | |
| parent | padre/madre | for COPPA / observer flows |
| guardian | tutor/a | |
| designer | diseñador/a | |

## Gradebook / academic

| English | Spanish | Notes |
|---|---|---|
| gradebook | libro de calificaciones | |
| dashboard | panel | "tablero" also valid; current es.json uses "Panel" |
| analytics | análisis / analíticas | current es.json uses "Analíticas" |
| rubric | rúbrica | |
| outcome | resultado de aprendizaje | shortened to "resultado" in nav contexts |
| late | tardío | |
| missing | faltante | |
| on time | a tiempo | |
| due | fecha límite / vence | "Vence hoy" / "Fecha límite" depending on context |
| points | puntos | |
| weight | peso / ponderación | |

## Actions

| English | Spanish | Notes |
|---|---|---|
| delete | eliminar | |
| edit | editar | |
| cancel | cancelar | |
| save | guardar | |
| confirm | confirmar | |
| close | cerrar | |
| back | atrás | |
| next | siguiente | |
| search | buscar | |
| filter | filtrar | |
| view | ver | |
| add | agregar | "añadir" also valid; existing es.json uses "Agregar" |
| remove | quitar | |
| create | crear | |
| publish | publicar | |
| unpublish | despublicar | |

## States / feedback

| English | Spanish | Notes |
|---|---|---|
| yes | sí | |
| no | no | |
| loading | cargando | |
| saving | guardando | |
| error | error | |
| success | éxito | |
| required | obligatorio | "requerido" also valid; existing es.json uses "Obligatorio" |
| optional | opcional | |
| visible | visible | |
| hidden | oculto | |
| published | publicado | |
| unpublished | no publicado | |
| draft | borrador | |
| pending | pendiente | |
| approved | aprobado | |
| rejected | rechazado | |

## Attendance

| English | Spanish | Notes |
|---|---|---|
| attendance | asistencia | |
| present | presente | |
| absent | ausente | |
| tardy / late | tarde | distinct from "tardío" (assignment-late) |
| excused | justificado/a | |
| unexcused | injustificado/a | |

## Time / date

| English | Spanish | Notes |
|---|---|---|
| today | hoy | |
| tomorrow | mañana | |
| yesterday | ayer | |
| this week | esta semana | |
| last week | la semana pasada | |
| start date | fecha de inicio | |
| end date | fecha de fin | |
| due date | fecha de entrega | |

## Punctuation / formatting conventions

- Spanish uses opening `¿` and `¡` for questions and exclamations.
  Mirror the English punctuation: `¿Olvidó su contraseña?`
- Capitalize only the first word of titles and sentences (sentence case).
  English title case ("Course Name") becomes "Nombre del curso".
- Decimal separator: `,` (e.g. `93,5%`). Thousands separator: `.` or
  non-breaking space. **Do NOT hand-format numbers in translation strings**
  — use `Intl.NumberFormat` at the call site.
- Date formatting: defer to `Intl.DateTimeFormat('es')` rather than
  hard-coding `5 de mayo de 2026` style strings in `es.json`.

## When the glossary disagrees with existing es.json

Existing `es.json` keys (translated pre-Wave-D.2) sometimes differ from this
glossary. **Do not retranslate existing keys** — that creates a moving target
for QA. Apply the glossary to NEW keys only, until a follow-up cleanup pass
reconciles legacy strings with the glossary.

## Follow-up cleanup pass (planned)

After D.1 and D.2 land, schedule a "consistency pass" that:
1. Lists every `es.json` value diverging from glossary recommendation.
2. Decides per-key whether to retranslate (breaking change) or grandfather.
3. Files migration notes in `web/src/i18n/CHANGELOG.md` so QA knows what
   shifted between releases.
