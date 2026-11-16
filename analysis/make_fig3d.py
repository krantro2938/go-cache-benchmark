"""3D surface figure: hit ratio and allocated bytes per request over λ × budget."""
import csv, sys
import numpy as np
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
from matplotlib.ticker import FuncFormatter
from PIL import Image

SUM, OUT = sys.argv[1], sys.argv[2]
plt.rcParams.update({'font.family': 'Liberation Serif', 'font.size': 8.5,
                     'mathtext.fontset': 'custom', 'mathtext.rm': 'Liberation Serif',
                     'mathtext.it': 'Liberation Serif:italic'})
rows = {r['config_id']: r for r in csv.DictReader(open(SUM)) if r['series'] == 'grid'}
LAM = [round(0.1 * i, 1) for i in range(11)]
MB = [4, 8, 16, 32, 64]
X, Y = np.meshgrid(LAM, np.log2(MB))
def Z(metric, scale):
    return np.array([[float(rows['grid_%dmb_lambda_%.1f' % (m, l)][metric + '_mean']) * scale for l in LAM] for m in MB])
comma = FuncFormatter(lambda v, p: ('%g' % round(v, 2)).replace('.', ','))

fig = plt.figure(figsize=(6.3, 3.1))
for k, (metric, scale, zlab, letter) in enumerate([
        ('hit_ratio', 100, 'Доля запросов из кэша, %', 'а)'),
        ('alloc_bytes_per_op', 1 / 1024, 'Выделено на запрос, КБ', 'б)')]):
    ax = fig.add_subplot(1, 2, k + 1, projection='3d')
    z = Z(metric, scale)
    ax.plot_surface(X, Y, z, cmap='Greys', vmin=z.min() - 0.6 * (z.max() - z.min()), vmax=z.max(),
                    edgecolor='black', linewidth=0.35, antialiased=True, alpha=0.95)
    ax.set_xlabel('Коэффициент λ', labelpad=1)
    ax.set_ylabel('Лимит памяти, МБ', labelpad=1)
    ax.set_yticks(np.log2(MB)); ax.set_yticklabels([str(m) for m in MB])
    ax.set_xticks([0, 0.25, 0.5, 0.75, 1.0]); ax.xaxis.set_major_formatter(comma)
    ax.zaxis.set_major_formatter(comma)
    ax.tick_params(axis='both', which='major', pad=0, labelsize=7.5)
    ax.view_init(elev=24, azim=-52)
    ax.set_box_aspect((1.15, 1.0, 0.8), zoom=0.86)
    for a in (ax.xaxis, ax.yaxis, ax.zaxis):
        a.pane.set_facecolor((1, 1, 1, 0)); a.pane.set_edgecolor('#999999')
        a._axinfo['grid'].update(color='#cfcfcf', linewidth=0.4)
    ax.text2D(0.02, 0.95, letter, transform=ax.transAxes, fontsize=10, fontweight='bold')
    ax.text2D(0.98, 0.86, zlab, transform=ax.transAxes, fontsize=8.5, ha='right')
fig.subplots_adjust(left=0.0, right=0.96, bottom=0.02, top=1.0, wspace=0.08)
fig.savefig(f'{OUT}/fig6.png', dpi=600)
im = Image.open(f'{OUT}/fig6.png').convert('L')
im.save(f'{OUT}/fig6.png', dpi=(600, 600)); im.save(f'{OUT}/fig6.tif', dpi=(600, 600), compression='tiff_lzw')
print(im.size)
