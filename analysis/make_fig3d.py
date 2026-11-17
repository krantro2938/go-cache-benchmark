import os
"""Fig. 5: banded 3D surfaces.
(а) share of requests served from cache over λ × memory limit (grid, mean of 5 runs);
(б) P99 latency in each of the 30 runs over λ × run number (λ series, raw values).
Usage: make_fig3d.py GRID_SUMMARY RESULTS_DIR OUT_DIR"""
import csv, glob, os, sys
import numpy as np
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
from matplotlib.patches import Patch
from mpl_toolkits.mplot3d.art3d import Poly3DCollection
from PIL import Image

GRID, RES, OUT = sys.argv[1], sys.argv[2], sys.argv[3]
plt.rcParams.update({'font.family': 'Liberation Serif', 'font.size': 8,
                     'mathtext.fontset': 'custom', 'mathtext.rm': 'Liberation Serif',
                     'mathtext.it': 'Liberation Serif:italic'})
fmt = lambda v: ('%g' % round(v, 2)).replace('.', ',')

def banded(ax, X, Y, Z, edges, shades):
    """Triangulated surface; every triangle is filled by the band of its mean height."""
    tris, cols = [], []
    ny, nx = Z.shape
    for i in range(ny - 1):
        for j in range(nx - 1):
            p = [(X[i, j], Y[i, j], Z[i, j]), (X[i, j + 1], Y[i, j + 1], Z[i, j + 1]),
                 (X[i + 1, j + 1], Y[i + 1, j + 1], Z[i + 1, j + 1]), (X[i + 1, j], Y[i + 1, j], Z[i + 1, j])]
            for t in ((p[0], p[1], p[2]), (p[0], p[2], p[3])):
                zc = np.mean([v[2] for v in t])
                k = min(np.searchsorted(edges, zc, side='right') - 1, len(shades) - 1)
                tris.append(t); cols.append(shades[max(k, 0)])
    pc = Poly3DCollection(tris, facecolors=cols, edgecolors='#4a4a4a', linewidths=0.5)
    ax.add_collection3d(pc)
    ax.set_xlim(X.min(), X.max()); ax.set_ylim(Y.min(), Y.max())
    ax.set_zlim(edges[0], edges[-1])

def style(ax):
    for a in (ax.xaxis, ax.yaxis, ax.zaxis):
        a.pane.set_facecolor((1, 1, 1, 0)); a.pane.set_edgecolor('#8a8a8a')
        a._axinfo['grid'].update(color='#bdbdbd', linewidth=0.5)
    ax.tick_params(axis='both', which='major', pad=0, labelsize=7)

def legend(fig, x, edges, shades, unit):
    hs = [Patch(facecolor=s, edgecolor='#4a4a4a', lw=0.5, label='%s–%s' % (fmt(a), fmt(b)))
          for a, b, s in zip(edges[:-1], edges[1:], shades)]
    fig.legend(handles=hs, loc='lower center', bbox_to_anchor=(x, -0.01), ncol=4, fontsize=6.5,
               frameon=False, handlelength=1.0, columnspacing=0.8, title=unit, title_fontsize=7)

SH9 = ['#f6f6f6', '#e3e3e3', '#cdcdcd', '#b3b3b3', '#979797', '#7a7a7a', '#5c5c5c', '#3d3d3d', '#1e1e1e']
fig = plt.figure(figsize=(6.3, 3.4))

# (а) hit ratio over λ × budget
rows = {r['config_id']: r for r in csv.DictReader(open(GRID)) if r['series'] == 'grid'}
LAM = [round(0.1 * i, 1) for i in range(11)]; MB = [4, 8, 16, 32, 64]
X, Y = np.meshgrid(LAM, np.log2(MB))
Z = np.array([[float(rows['grid_%dmb_lambda_%.1f' % (m, l)]['hit_ratio_mean']) * 100 for l in LAM] for m in MB])
ea = np.arange(45, 91, 5.0); shades = SH9
ax = fig.add_subplot(1, 2, 1, projection='3d')
banded(ax, X, Y, Z, ea, shades)
ax.set_xticks([0, 0.25, 0.5, 0.75, 1.0]); ax.set_xticklabels([fmt(v) for v in [0, 0.25, 0.5, 0.75, 1.0]])
ax.set_yticks(np.log2(MB)); ax.set_yticklabels([str(m) for m in MB])
ax.set_zticks(ea[::2]); ax.set_zticklabels([fmt(v) for v in ea[::2]])
ax.set_xlabel('Коэффициент λ', labelpad=-4); ax.set_ylabel('Лимит памяти, МБ', labelpad=-3)
ax.view_init(elev=22, azim=-58); ax.set_box_aspect((1.25, 1.0, 0.85), zoom=0.92); style(ax)
ax.text2D(0.02, 0.93, 'а)', transform=ax.transAxes, fontsize=10, fontweight='bold')
ax.text2D(0.12, 0.88, 'Доля запросов из кэша, %', transform=ax.transAxes, fontsize=7.5)
legend(fig, 0.25, ea, shades, 'Доля запросов из кэша, %')

# (б) P99 in each run over λ × run number
lam_ids = ['lambda_%.1f' % l for l in [0, 0.1, 0.3, 0.5, 0.7, 0.9, 1.0]]
p99 = {}
for f in glob.glob(os.path.join(RES, 'run_*', 'results.csv')):
    run = int(os.path.basename(os.path.dirname(f))[4:])
    for r in csv.DictReader(open(f)):
        if r['series'] == 'lambda':
            p99[(run, r['config_id'])] = float(r['p99_us'])
runs = sorted({k[0] for k in p99})
Xb, Yb = np.meshgrid(runs, range(len(lam_ids)))
Zb = np.array([[p99[(r, c)] for r in runs] for c in lam_ids])
eb = np.arange(1.0, 8.01, 1.0); shades_b = SH9[1:8]
bx = fig.add_subplot(1, 2, 2, projection='3d')
banded(bx, Xb.astype(float), Yb.astype(float), Zb, eb, shades_b)
bx.set_yticks([0, 2, 4, 6]); bx.set_yticklabels(['0', '0,3', '0,7', '1'])
bx.set_xticks([1, 5, 10, 15, 20, 25, 30]); bx.set_zticks(eb[1::2]); bx.set_zticklabels([fmt(v) for v in eb[1::2]])
bx.set_xlabel('Номер прогона', labelpad=-4); bx.set_ylabel('Коэффициент λ', labelpad=-3)
bx.view_init(elev=24, azim=-62); bx.set_box_aspect((1.6, 0.9, 0.85), zoom=0.86); style(bx)
bx.text2D(0.02, 0.93, 'б)', transform=bx.transAxes, fontsize=10, fontweight='bold')
bx.text2D(0.12, 0.88, 'Задержка P99, мкс', transform=bx.transAxes, fontsize=7.5)
legend(fig, 0.75, eb, shades_b, 'Задержка P99, мкс')

fig.subplots_adjust(left=0.0, right=1.0, bottom=0.17, top=1.0, wspace=0.0)
os.makedirs(OUT, exist_ok=True)
fig.savefig(f'{OUT}/fig6.png', dpi=600)
im = Image.open(f'{OUT}/fig6.png').convert('L')
im.save(f'{OUT}/fig6.png', dpi=(600, 600)); im.save(f'{OUT}/fig6.tif', dpi=(600, 600), compression='tiff_lzw')
print(im.size, Zb.min(), Zb.max())
