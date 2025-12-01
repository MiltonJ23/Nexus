#!/bin/bash

echo "=== 1. ARRÊT BRUTAL DES PROCESSUS ==="
sudo killall -9 nexus sleep 2>/dev/null
# On tue tout processus qui ressemblerait à un runc/init coincé
sudo killall -9 nsenter 2>/dev/null

echo "=== 2. NETTOYAGE SYSTÈME DE FICHIERS ==="
# Démontage forcé récursif (très important pour les namespaces bloqués)
sudo umount -R -f /run/nexus 2>/dev/null
sudo umount -R -f /var/lib/nexus/volumes 2>/dev/null
sudo umount -R -f /var/lib/nexus/images/alpine-base 2>/dev/null
# Nettoyage des dossiers
sudo rm -rf /run/nexus/*
sudo rm -rf /var/lib/nexus/volumes/*
sudo rm -f /tmp/nexus_debug_init.log

echo "=== 3. NETTOYAGE PÉRIPHÉRIQUES LOOP ==="
# Détache tous les fichiers .img montés
sudo losetup -D

echo "=== 4. RÉPARATION RÉSEAU (CRITIQUE) ==="
# Suppression des interfaces virtuelles
sudo ip link delete nexus0 2>/dev/null
# On supprime toutes les veth résiduelles
for veth in $(ip link show | grep veth | awk -F: '{print $2}' | cut -d@ -f1); do
    sudo ip link delete $veth 2>/dev/null
done

# PURGE DE LA TABLE DE ROUTAGE
echo "   -> Vérification des routes corrompues..."
# Si une route par défaut pointe vers 10.0.42.x, on la tue
sudo ip route flush table main
# On relance Netplan pour remettre la route par défaut correcte (celle de ta VM)
sudo netplan apply
# Alternative si netplan n'est pas utilisé : sudo systemctl restart systemd-networkd

echo "=== 5. NETTOYAGE CGROUPS ==="
sudo find /sys/fs/cgroup/nexus -mindepth 1 -delete 2>/dev/null
sudo rmdir /sys/fs/cgroup/nexus 2>/dev/null

echo "=== TERMINÉ. TEST DE PING... ==="
sleep 2
ping -c 2 8.8.8.8
