document.addEventListener("DOMContentLoaded", function () {
  var mount = document.getElementById("wc-player");
  if (!mount || typeof AsciinemaPlayer === "undefined") {
    return;
  }

  AsciinemaPlayer.create(mount.dataset.src, mount, {
    cols: Number(mount.dataset.cols || 100),
    rows: Number(mount.dataset.rows || 30),
    poster: mount.dataset.poster || "",
  });
});
