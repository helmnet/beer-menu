/** DisplayPage is the full-screen dark placeholder for the TV tap list at `/display`. */
export default function DisplayPage() {
  return (
    <main
      style={{
        position: "fixed",
        inset: 0,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        background: "#000",
        color: "#fff",
        fontSize: "4rem",
      }}
    >
      Tap list coming soon
    </main>
  );
}
