import sys
import matplotlib
matplotlib.use('TkAgg') 
import matplotlib.pyplot as plt
import pandas as pd

fname = sys.argv[1]

df = pd.read_csv(fname)
plt.plot(df[df.columns[0]], df[df.columns[1]])
plt.savefig('plot.png')
