"""Figures 2-5 of the paper from results/summary.csv (grayscale, 600 dpi)."""
import csv, sys
import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
from matplotlib.ticker import FuncFormatter
from PIL import Image

SUM = sys.argv[1]
OUT = sys.argv[2]
plt.rcParams.update({'font.family': 'Liberation Serif', 'font.size': 9, 'axes.linewidth': 0.7,
                     'mathtext.fontset': 'custom', 'mathtext.rm': 'Liberation Serif',
                     'mathtext.it': 'Liberation Serif:italic', 'legend.frameon': False})
rows = list(csv.DictReader(open(SUM)))
def get(cfg, cache, m):
    for r in rows:
        if r['config_id'] == cfg and r['cache'] == cache:
            return float(r[m + '_mean']), float(r[m + '_ci95'])
    raise KeyError((cfg, cache, m))

ST = {
 'GCAware':          dict(label='Предложенный метод', color='#000000', ls='-',  marker='o', mfc='#000000', ms=4.5),
 'Adaptive_TinyLFU': dict(label='Adaptive TinyLFU',   color='#404040', ls='--', marker='s', mfc='white',   ms=4.5),
 'Ristretto':        dict(label='Ristretto',          color='#5a5a5a', ls='-.', marker='^', mfc='white',   ms=5),
 'BigCache':         dict(label='BigCache',           color='#7a7a7a', ls=':',  marker='v', mfc='#7a7a7a', ms=5),
}
ORDER = ['GCAware', 'Adaptive_TinyLFU', 'Ristretto', 'BigCache']
comma = FuncFormatter(lambda v, p: ('%g' % v).replace('.', ','))

def line(ax, x, cfgs, cache, m, scale=1.0):
    s = ST[cache]
    ys = [get(c, cache, m)[0] * scale for c in cfgs]
    ax.plot(x, ys, color=s['color'], ls=s['ls'], marker=s['marker'], ms=s['ms'], mfc=s['mfc'],
            mec=s['color'], mew=1.0, lw=1.1, label=s['label'])

def panel(ax, letter):
    ax.text(-0.13, 1.03, letter, transform=ax.transAxes, fontsize=10, fontweight='bold', va='bottom')
    ax.grid(True, color='#d0d0d0', lw=0.4)
    ax.yaxis.set_major_formatter(comma)

def save(fig, name):
    fig.savefig(f'{OUT}/{name}.png', dpi=600)
    plt.close(fig)
    im = Image.open(f'{OUT}/{name}.png').convert('L')
    im.save(f'{OUT}/{name}.png', dpi=(600, 600))
    im.save(f'{OUT}/{name}.tif', dpi=(600, 600), compression='tiff_lzw')
    print(name, im.size)

VAL = ['val_256b', 'val_1kb', 'val_4kb', 'val_16kb', 'val_64kb']; VALX = ['0,25', '1', '4', '16', '64']
SKEW = ['skew_0.80', 'skew_0.90', 'skew_0.95', 'skew_0.99']; SKEWX = [0.80, 0.90, 0.95, 0.99]
SIZE = ['size_64mb', 'size_128mb', 'size_256mb', 'size_512mb', 'size_1024mb']; SIZEX = ['64', '128', '256', '512', '1024']

# Fig. 2: hit ratio vs object size and vs skew
fig, (a, b) = plt.subplots(1, 2, figsize=(6.3, 2.4))
for c in ORDER:
    line(a, range(5), VAL, c, 'hit_ratio', 100)
    line(b, SKEWX, SKEW, c, 'hit_ratio', 100)
a.set_xticks(range(5)); a.set_xticklabels(VALX)
a.set_xlabel('Размер объекта, КБ'); a.set_ylabel('Коэффициент попаданий, %')
b.set_xlabel('Степень скошенности $s$'); b.set_ylabel('Коэффициент попаданий, %')
b.xaxis.set_major_formatter(comma); b.set_xticks(SKEWX)
panel(a, 'а)'); panel(b, 'б)')
h, l = a.get_legend_handles_labels()
fig.legend(h, l, loc='upper center', ncol=4, fontsize=8.5, bbox_to_anchor=(0.5, 1.0))
fig.tight_layout(rect=(0, 0, 1, 0.91)); save(fig, 'fig2')

# Fig. 3: throughput and P99 vs object size
fig, (a, b) = plt.subplots(1, 2, figsize=(6.3, 2.4))
for c in ORDER:
    line(a, range(5), VAL, c, 'ops_per_sec', 1e-6)
    line(b, range(5), VAL, c, 'p99_us')
for ax in (a, b):
    ax.set_xticks(range(5)); ax.set_xticklabels(VALX); ax.set_xlabel('Размер объекта, КБ')
a.set_ylabel('Млн операций в секунду')
b.set_yscale('log'); b.set_ylabel('Задержка P99, мкс')
panel(a, 'а)'); panel(b, 'б)')
b.yaxis.set_major_formatter(comma); b.yaxis.set_minor_formatter(FuncFormatter(lambda v, p: ''))
h, l = a.get_legend_handles_labels()
fig.legend(h, l, loc='upper center', ncol=4, fontsize=8.5, bbox_to_anchor=(0.5, 1.0))
fig.tight_layout(rect=(0, 0, 1, 0.91)); save(fig, 'fig3')

# Fig. 4: live memory relative to the budget; GC pause per cycle
fig, (a, b) = plt.subplots(1, 2, figsize=(6.3, 2.4))
budgets = [64, 128, 256, 512, 1024]
for c in ORDER:
    s = ST[c]
    ys = [get(cfg, c, 'live_mb')[0] / bud for cfg, bud in zip(SIZE, budgets)]
    a.plot(range(5), ys, color=s['color'], ls=s['ls'], marker=s['marker'], ms=s['ms'], mfc=s['mfc'],
           mec=s['color'], mew=1.0, lw=1.1, label=s['label'])
a.axhline(1.0, color='black', lw=0.6)
a.set_xticks(range(5)); a.set_xticklabels(SIZEX); a.set_xlabel('Лимит памяти кэша, МБ')
a.set_ylabel('Живые данные / лимит')
a.legend(fontsize=7.5, loc='upper right', handlelength=2.6)
cfgs = ['size_512mb', 'val_1kb', 'skew_0.90', 'mixed_16mb']; labs = ['size_512mb', 'val_1kb', 'skew_0.90', 'mixed_16mb']
hatches = ['', '////', '....', 'xxxx']; fills = ['#000000', 'white', 'white', '#9a9a9a']
w = 0.2
for i, c in enumerate(ORDER):
    vals = [get(cfg, c, 'gc_pause_us')[0] for cfg in cfgs]
    errs = [get(cfg, c, 'gc_pause_us')[1] for cfg in cfgs]
    b.bar([j + (i - 1.5) * w for j in range(4)], vals, w, yerr=errs, color=fills[i], edgecolor='black',
          hatch=hatches[i], lw=0.6, error_kw=dict(lw=0.6, capsize=1.5), label=ST[c]['label'])
b.set_xticks(range(4)); b.set_xticklabels(labs, fontsize=8)
b.set_ylabel('Пауза GC на цикл, мкс')
panel(a, 'а)'); panel(b, 'б)')
h, l = b.get_legend_handles_labels()
fig.legend(h, l, loc='upper center', ncol=4, fontsize=8.5, bbox_to_anchor=(0.5, 1.0))
fig.tight_layout(rect=(0, 0, 1, 0.91)); save(fig, 'fig4')

# Fig. 5: lambda sweep
LAM = [0.0, 0.1, 0.3, 0.5, 0.7, 0.9, 1.0]; LC = ['lambda_%.1f' % l for l in LAM]
fig, (a, b) = plt.subplots(1, 2, figsize=(6.3, 2.3))
hr = [get(c, 'GCAware', 'hit_ratio')[0] * 100 for c in LC]
bh = [get(c, 'GCAware', 'byte_hit_ratio')[0] * 100 for c in LC]
a.plot(LAM, hr, 'k-o', ms=4.5, lw=1.1, label='доля запросов')
a.plot(LAM, bh, color='#555555', ls='--', marker='s', mfc='white', ms=4.5, lw=1.1, label='доля байтов')
a.set_xlabel('Коэффициент λ'); a.set_ylabel('Обслужено из кэша, %'); a.legend(fontsize=8.5, loc='lower right')
ab = [get(c, 'GCAware', 'alloc_bytes_per_op')[0] / 1024 for c in LC]
b.plot(LAM, ab, 'k-o', ms=4.5, lw=1.1)
b.set_xlabel('Коэффициент λ'); b.set_ylabel('Выделено памяти на запрос, КБ')
for ax in (a, b):
    ax.xaxis.set_major_formatter(comma); ax.set_xticks([0, 0.1, 0.3, 0.5, 0.7, 0.9, 1.0])
panel(a, 'а)'); panel(b, 'б)')
fig.tight_layout(); save(fig, 'fig5')
