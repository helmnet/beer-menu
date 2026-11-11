import type { CSSProperties } from "react";
import type { Menu, MenuBeer, MenuTap, OnDeckItem } from "../api";
// Loads the @font-face rules; displayOf has already checked every font key.
import "../fonts";
import type { Page, TvDisplay } from "../tv";
import BeerArt from "./BeerArt";
import "../pages/DisplayPage.css";

/**
 * Facts is one line of a beer's details, each only when asked for and known: style (rows only;
 * cards give it its own line), ABV as "N%", IBU as "N IBU" and SRM as "SRM N", rounded. Nothing
 * when none is shown, so no empty line.
 */
function Facts({ beer, style = false, abv, ibu, srm }: { beer: MenuBeer; style?: boolean; abv: boolean; ibu: boolean; srm: boolean }) {
  const parts = [
    style && beer.style !== null && beer.style,
    abv && beer.abv !== null && `${beer.abv}%`,
    ibu && beer.ibu !== null && `${beer.ibu} IBU`,
    // The API leaves srm out when unset; null is treated the same.
    srm && beer.srm != null && `SRM ${Math.round(beer.srm)}`,
  ].filter((p) => p !== false);
  if (parts.length === 0) return null;
  return (
    <p className="facts">
      {parts.map((p, i) => (
        <span key={i}>{p}</span>
      ))}
    </p>
  );
}

/**
 * TapText is a tap beer's name and the details its switches in `display` turn on, each only when
 * known: style (its own line on a card, first on the Facts line in a row), the Facts line, and
 * the description, drawn as text and clamped by CSS.
 */
function TapText({ beer, display, row }: { beer: MenuBeer; display: TvDisplay; row: boolean }) {
  return (
    <>
      <p className="name">{beer.name}</p>
      {!row && display.show_style && beer.style !== null && <p className="style">{beer.style}</p>}
      <Facts beer={beer} style={row && display.show_style} abv={display.show_abv} ibu={display.show_ibu} srm={display.show_srm} />
      {display.show_description && beer.description && <p className="description">{beer.description}</p>}
    </>
  );
}

/** TapList is the taps of one page as cards, or as rows when `row`. */
function TapList({ taps, display, row }: { taps: MenuTap[]; display: TvDisplay; row: boolean }) {
  return (
    <ul className={row ? "rows" : "cards"} aria-label="Taps">
      {taps.map((t, i) => {
        const art = <BeerArt onTap color={t.beer?.color ?? null} name={t.beer?.name ?? "Empty"} level={t.keg?.level} height={240} />;
        const label = <p className="label">{t.label}</p>;
        const text = t.beer ? <TapText beer={t.beer} display={display} row={row} /> : <p className="name">Empty</p>;
        // Labels need not be unique and the list is replaced whole each check.
        return row ? (
          <li key={i} className="row">
            {art}
            <div className="text">
              {label}
              {text}
            </div>
          </li>
        ) : (
          <li key={i} className="card">
            {label}
            {art}
            <div className="text">{text}</div>
          </li>
        );
      })}
    </ul>
  );
}

/**
 * DeckList is the on deck items of one page as cards, or as rows when `row`: name, style, ABV and
 * status whatever the detail switches (FR-013).
 */
function DeckList({ items, row }: { items: OnDeckItem[]; row: boolean }) {
  return (
    <ul className={row ? "rows" : "cards"} aria-label="On deck">
      {items.map((d, i) => (
        <li key={i} className={row ? "row" : "card"}>
          <BeerArt kegShape={d.keg?.shape} color={d.beer.color} name={d.beer.name} level={d.keg?.level} height={240} />
          <div className="text">
            <p className="name">{d.beer.name}</p>
            {!row && d.beer.style !== null && <p className="style">{d.beer.style}</p>}
            <Facts beer={d.beer} style={row} abv ibu={false} srm={false} />
            {d.status && <p className="status">{d.status === "fermenting" ? "Fermenting" : "Conditioning"}</p>}
          </div>
        </li>
      ))}
    </ul>
  );
}

/**
 * TvView is the TV drawing of one page, the one drawing shared by the TV (`DisplayPage`) and the
 * Display Settings preview, so the two cannot drift apart. It is pure: no timers and no fetch.
 * `menu` is null before the first good check, which draws an empty page; `pages` is
 * `pagesOf(menu, display.layout)` and `page` the index to draw; the "offline" notice shows once
 * `failures` reaches `offlineAfter`. The root is `main`, or a `div` when `preview`.
 *
 * Enabled taps show as cards in menu order: label, a pint glass filled to the keg level, beer
 * name, then each detail whose `show_*` switch is on and that the beer has: style, ABV, IBU, SRM
 * ("SRM N", rounded) and the description, clamped to 2 lines; an empty tap shows "Empty" and an
 * empty glass. On deck pages show an "On deck" heading over cards: waiting kegs in their keg
 * shape filled to their level, and beers still being made as a full corny keg with their status;
 * on deck cards always show name, style, ABV and status, whatever the switches (FR-013). In
 * `grid` the class is `taps-s` for 1 to 4 cards on the page and `taps-m` for 5 to 8. `list`
 * (class `layout-list`) draws the same pages as rows: the same small art on every row, so the
 * gauge reads the same, then label, name and details. `columns` (class `layout-columns`) draws
 * an "On tap" column beside an "On deck" column; an empty list says "Nothing on tap" or "Nothing
 * on deck" in its column, and a shorter column is blank after its last row. With more than one
 * page a "N / M" indicator shows. "Nothing on tap" fills the page only when both lists are empty.
 * No level number is shown; the level is only the fill and the art's label.
 *
 * `display` sets the theme (class `light` for the light palette in DisplayPage.css, else dark)
 * and the title, one heading line above the cards when non-empty, drawn as text, aligned by
 * `title_align` (class `align-center` or `align-right`). The text font's class (`font-<key>`,
 * as in FONTS) is on the root; the title adds its own font class when `title_font` differs, so
 * it never takes the text font. A set `title_color` or `text_color` reaches the stylesheet only
 * as the custom property `--title-color` or `--tv-text` on the root, set through the CSSOM,
 * which the CSP allows; the values are validated "#rrggbb", so nothing else can reach a style.
 * With theme colors (both "") the root gets no style from React. No style attributes in markup
 * (CSP): classes from DisplayPage.css.
 */
export default function TvView({
  menu,
  display,
  pages,
  page,
  failures,
  offlineAfter,
  preview = false,
}: {
  menu: Menu | null;
  display: TvDisplay;
  pages: Page[];
  page: number;
  failures: number;
  offlineAfter: number;
  preview?: boolean;
}) {
  // The preview sits inside the admin page's own main, and a page has one main landmark.
  const Root = preview ? "div" : "main";
  const { title, theme, layout, title_align, title_font, text_font, title_color, text_color } = display;
  // React writes these through the CSSOM (style.setProperty), which the CSP allows; a style
  // attribute in markup it would block. displayOf lets only "#rrggbb" through (research R3, R4).
  const colors: Record<string, string> = {};
  if (title_color) colors["--title-color"] = title_color;
  if (text_color) colors["--tv-text"] = text_color;
  // The title inherits the text font unless it has its own; left alignment is the default.
  const titleClass = ["title", title_align !== "left" && `align-${title_align}`, title_font !== text_font && `font-${title_font}`]
    .filter(Boolean)
    .join(" ");
  const cur = pages[page];
  const row = layout !== "grid";
  const size = row ? `layout-${layout}` : (cur?.taps ?? cur?.onDeck ?? []).length <= 4 ? "taps-s" : "taps-m";

  return (
    <Root
      className={`display ${size} font-${text_font}${theme === "light" ? " light" : ""}`}
      style={colors as CSSProperties}
    >
      {title && <h1 className={titleClass}>{title}</h1>}
      {menu && pages.length === 0 && <p className="nothing">Nothing on tap</p>}
      {layout === "columns" && menu && cur ? (
        <div className="columns">
          <section className="column">
            <h2 className="heading">On tap</h2>
            {menu.taps.length === 0 ? (
              <p className="nothing">Nothing on tap</p>
            ) : (
              !!cur.taps?.length && <TapList taps={cur.taps} display={display} row />
            )}
          </section>
          <section className="column">
            <h2 className="heading">On deck</h2>
            {menu.on_deck.length === 0 ? (
              <p className="nothing">Nothing on deck</p>
            ) : (
              !!cur.onDeck?.length && <DeckList items={cur.onDeck} row />
            )}
          </section>
        </div>
      ) : (
        <>
          {cur?.taps && <TapList taps={cur.taps} display={display} row={row} />}
          {cur?.onDeck && (
            <>
              <h2 className="heading">On deck</h2>
              <DeckList items={cur.onDeck} row={row} />
            </>
          )}
        </>
      )}
      {pages.length > 1 && (
        <div className="pager">
          {page + 1} / {pages.length}
        </div>
      )}
      {failures >= offlineAfter && (
        <div className="offline" role="status">
          offline
        </div>
      )}
    </Root>
  );
}
