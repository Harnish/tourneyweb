(function () {
  function makeSortable(table) {
    var ths = Array.from(table.querySelectorAll('tr:first-child th'));
    if (!ths.length) return;
    ths.forEach(function (th) { th._label = th.textContent.trim(); });

    var activeCol = -1, ascending = true;

    ths.forEach(function (th, col) {
      th.style.cursor = 'pointer';
      th.addEventListener('click', function () {
        if (activeCol === col) {
          ascending = !ascending;
        } else {
          activeCol = col;
          ascending = true;
        }
        var isNum = th.dataset.sort === 'number';
        var rows = Array.from(table.rows).slice(1);
        rows.sort(function (a, b) {
          var av = a.cells[col] ? a.cells[col].textContent.trim() : '';
          var bv = b.cells[col] ? b.cells[col].textContent.trim() : '';
          if (isNum) return (parseFloat(av) || 0) - (parseFloat(bv) || 0);
          return av.localeCompare(bv, undefined, {sensitivity: 'base'});
        });
        if (!ascending) rows.reverse();
        rows.forEach(function (r) { r.parentNode.appendChild(r); });
        ths.forEach(function (h) { h.textContent = h._label; });
        th.textContent = th._label + (ascending ? ' ↑' : ' ↓');
      });
    });
  }

  document.addEventListener('DOMContentLoaded', function () {
    document.querySelectorAll('table.tw-sortable').forEach(makeSortable);
  });
}());
