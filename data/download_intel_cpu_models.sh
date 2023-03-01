#!/bin/bash

sandy="https://ark.intel.com/content/www/us/en/ark/products/codename/29900/products-formerly-sandy-bridge.html#@nofilter"
sandy_ep="https://ark.intel.com/content/www/us/en/ark/products/codename/64276/products-formerly-sandy-bridge-ep.html#@nofilter"
sandy_en="https://ark.intel.com/content/www/us/en/ark/products/codename/64275/products-formerly-sandy-bridge-en.html#@nofilter"
ivy="https://ark.intel.com/content/www/us/en/ark/products/codename/29902/products-formerly-ivy-bridge.html#@nofilter"
ivy_ep="https://ark.intel.com/content/www/us/en/ark/products/codename/68926/products-formerly-ivy-bridge-ep.html#@nofilter"
haswell="https://ark.intel.com/content/www/us/en/ark/products/codename/42174/products-formerly-haswell.html#@nofilter"
broadwell="https://ark.intel.com/content/www/us/en/ark/products/codename/38530/products-formerly-broadwell.html#@nofilter"
skylake="https://ark.intel.com/content/www/us/en/ark/products/codename/37572/products-formerly-skylake.html#@nofilter"
cascade="https://ark.intel.com/content/www/us/en/ark/products/codename/124664/products-formerly-cascade-lake.html#@nofilter"
coffee="https://ark.intel.com/content/www/us/en/ark/products/codename/97787/products-formerly-coffee-lake.html#@nofilter"
alder="https://ark.intel.com/content/www/us/en/ark/products/codename/147470/products-formerly-alder-lake.html#@nofilter"
icelake="https://ark.intel.com/content/www/us/en/ark/products/codename/74979/products-formerly-ice-lake.html#@nofilter"
spr="https://ark.intel.com/content/www/us/en/ark/products/codename/126212/products-formerly-sapphire-rapids.html#@nofilter"
spr_hbm="https://ark.intel.com/content/www/us/en/ark/products/codename/230303/products-formerly-sapphire-rapids-hbm.html#@nofilter"

download_arch() {
   url=$1
   arch=$2
   models=$(curl $1 | pup 'td[class="ark-product-name ark-accessible-color component"] text{}' |grep Processor | grep -Po "( [-a-zA-Z0-9]+[0-9]+[A-Z]* )")
   for m in $models; do
	   echo $m,$arch >> $file
   done
}

[[ "$(command -v pup)" ]] || { echo "Please install pup from github.com/ericchiang/pup first." 1>&2 ; exit 1; }

file="cpu_model.csv"
echo "Model,Architecture" > $file

download_arch $sandy "Sandy Bridge"
download_arch $sandy_ep "Sandy Bridge"
download_arch $sandy_en "Sandy Bridge"
download_arch $ivy "Ivy Bridge"
download_arch $ivy_ep "Ivy Bridge"
download_arch $haswell "Haswell"
download_arch $broadwell "Broadwell"
download_arch $skylake "Sky Lake"
download_arch $cascade "Cascade Lake"
download_arch $coffee "Coffee Lake"
download_arch $alder "Alder Lake"
download_arch $icelake "Ice Lake"
download_arch $spr "Sapphire Rapids"
download_arch $spr_hbm "Sapphire Rapids"
