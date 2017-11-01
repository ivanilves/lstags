#!/bin/bash
# Workaround because the Makefile doesn't recognize the bash
# This should be fixed in the Makefile and not in the directory    
ln -nfs /bin/bash /bin/sh
make all
