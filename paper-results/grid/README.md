Lambda x budget grid for the 3D figure (Fig. 5 of the revised paper): `SERIES=grid`, seeds 1-5, run in a cloud VM (2 vCPU). Hit ratio and allocated bytes per request do not depend on the hardware; at 16 MB and lambda >= 0.1 they match the laptop runs within 0.1 pp and 0.6%.

Figure: `python3 analysis/make_fig3d.py paper-results/grid/summary.csv <out_dir>`
