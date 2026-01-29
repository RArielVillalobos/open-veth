package orchestrator

import (
	"fmt"
	"runtime"
	
	"open-veth/internal/models"

	"github.com/vishvananda/netlink"
	"github.com/vishvananda/netns"
)

// NetworkManager maneja la configuración de red a nivel de kernel
type NetworkManager struct{}

// NewNetworkManager crea una nueva instancia
func NewNetworkManager() *NetworkManager {
	return &NetworkManager{}
}

// runInNs ejecuta una función dentro del namespace de red de un proceso (PID)
// Se encarga de bloquear el hilo y restaurar el namespace original al finalizar.
func (nm *NetworkManager) runInNs(pid int, action func() error) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// Guardar el namespace original
	origns, err := netns.Get()
	if err != nil {
		return fmt.Errorf("error obteniendo netns original: %v", err)
	}
	defer origns.Close()

	// Obtener handle del namespace destino
	targetNs, err := netns.GetFromPid(pid)
	if err != nil {
		return fmt.Errorf("error obteniendo ns del pid %d: %v", pid, err)
	}
	defer targetNs.Close()

	// Cambiar al namespace destino
	if err := netns.Set(targetNs); err != nil {
		return fmt.Errorf("error cambiando al ns %d: %v", pid, err)
	}

	// Ejecutar la acción
	err = action()

	// Volver al namespace original
	// Es crítico hacer esto antes de retornar, incluso si action() falló
	if errSwitchBack := netns.Set(origns); errSwitchBack != nil {
		// Si fallamos en volver, estamos en un estado inconsistente (panic worthy en algunos casos)
		return fmt.Errorf("CRÍTICO: error volviendo al ns original (error acción: %v): %v", err, errSwitchBack)
	}

	return err
}

// CreateLink crea un cable virtual (veth pair) y conecta dos namespaces (PIDs)
func (nm *NetworkManager) CreateLink(link models.Link, pidSource, pidTarget int) error {
	// Nombres temporales en el host
	hostVethNameSource := fmt.Sprintf("veth%s_s", link.ID[:5])
	hostVethNameTarget := fmt.Sprintf("veth%s_t", link.ID[:5])

	// 1. Crear el veth pair en el host (requiere estar en el ns original, asumimos que lo estamos)
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name:   hostVethNameSource,
			TxQLen: 1000,
		},
		PeerName: hostVethNameTarget,
	}

	if err := netlink.LinkAdd(veth); err != nil {
		return fmt.Errorf("error creando veth pair: %v", err)
	}

	// Función helper para mover y renombrar
	moveAndConfigure := func(ifaceHostName, ifaceContainerName string, pid int) error {
		// Buscamos la interfaz en el host
		linkRef, err := netlink.LinkByName(ifaceHostName)
		if err != nil {
			return err
		}

		// Obtenemos el NS del destino solo para moverla (netlink.LinkSetNsFd necesita el FD)
		targetNs, err := netns.GetFromPid(pid)
		if err != nil {
			return err
		}
		defer targetNs.Close()

		// Movemos la interfaz
		if err := netlink.LinkSetNsFd(linkRef, int(targetNs)); err != nil {
			return fmt.Errorf("error moviendo interfaz: %v", err)
		}

		// Ahora entramos al NS para renombrarla y levantarla
		return nm.runInNs(pid, func() error {
			l, err := netlink.LinkByName(ifaceHostName)
			if err != nil {
				return fmt.Errorf("interfaz movida no encontrada: %v", err)
			}
			
			if err := netlink.LinkSetName(l, ifaceContainerName); err != nil {
				return fmt.Errorf("error renombrando: %v", err)
			}
			
			// Re-buscamos por nuevo nombre
			l, err = netlink.LinkByName(ifaceContainerName)
			if err != nil {
				return err
			}
			
			return netlink.LinkSetUp(l)
		})
	}

	// 2. Mover extremos
	if err := moveAndConfigure(hostVethNameSource, link.SourceInt, pidSource); err != nil {
		return fmt.Errorf("fallo configurando source: %v", err)
	}
	if err := moveAndConfigure(hostVethNameTarget, link.TargetInt, pidTarget); err != nil {
		return fmt.Errorf("fallo configurando target: %v", err)
	}

	fmt.Printf("Link creado: %s (%s) <--> %s (%s)\n", 
		link.SourceID, link.SourceInt, link.TargetID, link.TargetInt)

	return nil
}

// CreateBridge crea un Linux Bridge en el host (actúa como Switch)
func (nm *NetworkManager) CreateBridge(bridgeName string) error {
	// Verificar si ya existe
	if _, err := netlink.LinkByName(bridgeName); err == nil {
		return nil // Ya existe, idempotente
	}

	br := &netlink.Bridge{
		LinkAttrs: netlink.LinkAttrs{
			Name: bridgeName,
		},
	}

	if err := netlink.LinkAdd(br); err != nil {
		return fmt.Errorf("error creando bridge %s: %v", bridgeName, err)
	}

	// Levantar el bridge
	if err := netlink.LinkSetUp(br); err != nil {
		return fmt.Errorf("error levantando bridge %s: %v", bridgeName, err)
	}

	fmt.Printf("Bridge (Switch) creado: %s\n", bridgeName)
	return nil
}

// ConnectNodeToBridge conecta un contenedor (PID) a un Bridge en el host
func (nm *NetworkManager) ConnectNodeToBridge(pid int, containerIface, bridgeName string) error {
	// Generar nombres cortos y seguros para evitar limite de 15 chars de Linux
	// Formato: v<PID>-<Primeras3LetrasIface>
	// Ej: PID=1234, Iface=eth1 -> v1234-eth
	suffix := containerIface
	if len(suffix) > 3 {
		suffix = suffix[:3]
	}
	
	hostVethName := fmt.Sprintf("v%d-%s", pid, suffix)
	containerVethTemp := hostVethName + "c" // temp name for container side

	// 1. Crear veth pair
	veth := &netlink.Veth{
		LinkAttrs: netlink.LinkAttrs{
			Name: hostVethName,
		},
		PeerName: containerVethTemp,
	}

	if err := netlink.LinkAdd(veth); err != nil {
		return fmt.Errorf("error creando veth para bridge: %v", err)
	}

	// 2. Conectar lado Host al Bridge
	bridgeLink, err := netlink.LinkByName(bridgeName)
	if err != nil {
		return fmt.Errorf("bridge %s no encontrado: %v", bridgeName, err)
	}

	hostLink, err := netlink.LinkByName(hostVethName)
	if err != nil {
		return err
	}

	if err := netlink.LinkSetMaster(hostLink, bridgeLink); err != nil {
		return fmt.Errorf("error conectando veth %s al bridge: %v", hostVethName, err)
	}

	if err := netlink.LinkSetUp(hostLink); err != nil {
		return fmt.Errorf("error levantando veth host: %v", err)
	}

	// 3. Mover lado Container al Namespace
	// Reutilizamos lógica similar a CreateLink, pero simplificada inline o extraemos funcion Move
	// Por ahora copiamos la logica de movimiento:
	
	targetNs, err := netns.GetFromPid(pid)
	if err != nil {
		return err
	}
	defer targetNs.Close()

	peerLink, err := netlink.LinkByName(containerVethTemp)
	if err != nil {
		return err
	}

	if err := netlink.LinkSetNsFd(peerLink, int(targetNs)); err != nil {
		return fmt.Errorf("error moviendo interfaz al container: %v", err)
	}

	// 4. Configurar dentro del Container
	return nm.runInNs(pid, func() error {
		l, err := netlink.LinkByName(containerVethTemp)
		if err != nil {
			return fmt.Errorf("interfaz movida no encontrada: %v", err)
		}
		
		if err := netlink.LinkSetName(l, containerIface); err != nil {
			return fmt.Errorf("error renombrando a %s: %v", containerIface, err)
		}
		
		// Re-buscar
		l, err = netlink.LinkByName(containerIface)
		if err != nil { return err }
		
		return netlink.LinkSetUp(l)
	})
}

// SetInterfaceIP asigna una IP/CIDR a una interfaz dentro de un namespace (PID)
func (nm *NetworkManager) SetInterfaceIP(pid int, ifaceName string, ipCidr string) error {
	return nm.runInNs(pid, func() error {
		// 1. Buscar la interfaz
		link, err := netlink.LinkByName(ifaceName)
		if err != nil {
			return fmt.Errorf("interfaz %s no encontrada: %v", ifaceName, err)
		}

		// 2. Parsear la IP
		addr, err := netlink.ParseAddr(ipCidr)
		if err != nil {
			return fmt.Errorf("formato IP incorrecto %s: %v", ipCidr, err)
		}

		// 3. Asignar la IP
		if err := netlink.AddrAdd(link, addr); err != nil {
			return fmt.Errorf("error asignando IP: %v", err)
		}

		fmt.Printf("IP asignada en PID %d: %s -> %s\n", pid, ifaceName, ipCidr)
		return nil
	})
}