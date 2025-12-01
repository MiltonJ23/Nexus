# 1. Tuer tous les processus liés à Nexus ou au conteneur
sudo killall sleep nexus 2> /dev/null
# Forcer l'arrêt si des processus zombies résistent
sudo kill -9 $(pidof nexus) 2> /dev/null
sudo kill -9 $(pidof sleep) 2> /dev/null

# 2. Nettoyer les interfaces réseau virtuelles
# Remplace 'force-test', 'node-01', etc par tous les noms utilisés, ou utilise cette boucle :
for veth in $(ip link show | grep veth | awk -F: '{print $2}'); do sudo ip link delete $veth; done
for br in $(ip link show | grep nexus0 | awk -F: '{print $2}'); do sudo ip link delete $br; done

# 3. Démonter les volumes et supprimer les loopbacks
# D'abord on démonte tout ce qui pourrait être monté dans /run/nexus ou /data
sudo umount -R /run/nexus 2> /dev/null
sudo umount -R /var/lib/nexus/volumes 2> /dev/null
# On détache tous les périphériques loop (les fichiers .img montés comme disques)
sudo losetup -D

# 4. Supprimer les fichiers d'état et les Cgroups
sudo rm -rf /run/nexus/*
sudo rm -rf /var/lib/nexus/volumes/* # Attention: efface les données des volumes créés
# Nettoyage des Cgroups (peut échouer si 'device or resource busy', insister ou ignorer)
sudo find /sys/fs/cgroup/nexus -mindepth 1 -delete 2> /dev/null
sudo rmdir /sys/fs/cgroup/nexus 2> /dev/null

# 5. Nettoyer les fichiers de log temporaires
sudo rm /tmp/nexus_debug_init.log 2> /dev/null