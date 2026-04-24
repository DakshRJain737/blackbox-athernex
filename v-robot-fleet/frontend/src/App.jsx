import React, { useState, useEffect } from 'react';
import './App.css';

const ROBOT_DATA = {
  'WH-NAV-01': { role: 'Navigator', name: 'Pathfinder Alpha', icon: '🧭' },
  'WH-NAV-02': { role: 'Navigator', name: 'Pathfinder Beta', icon: '🧭' },
  'WH-CAR-01': { role: 'Carrier', name: 'HeavyLift One', icon: '🚚' },
  'WH-CAR-02': { role: 'Carrier', name: 'HeavyLift Two', icon: '🚚' },
  'WH-SCN-01': { role: 'Scanner', name: 'Sentinel Prime', icon: '📡' },
  'WH-SCN-02': { role: 'Scanner', name: 'Sentinel Echo', icon: '📡' },
  'WH-RUN-01': { role: 'ChargeRunner', name: 'DockDasher One', icon: '⚡' },
  'WH-RUN-02': { role: 'ChargeRunner', name: 'DockDasher Two', icon: '⚡' },
};

const CRITICAL_THRESHOLDS = {
  battery: { critical: 15, warning: 30 },
  motor_temp: { critical: 75, warning: 65 },
  proximity: { critical: 20, warning: 50 },
  vibration: { critical: 80, warning: 60 },
  scan_accuracy: { critical: 50, warning: 75 },
  axle_stress: { critical: 85, warning: 70 },
};

function App() {
  const [robotSensors, setRobotSensors] = useState({});
  const [elapsedTime, setElapsedTime] = useState(0);

  useEffect(() => {
    const startTime = Date.now();
    const ws = new WebSocket('ws://localhost:9000/ws');

    ws.onopen = () => {
      console.log('Connected to relay');
    };

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data);
        const topic = msg.topic;
        const value = parseFloat(msg.value);
        const [robotID, sensorName] = topic.split('.');
        if (robotID && sensorName) {
          setRobotSensors((prev) => ({
            ...prev,
            [robotID]: {
              ...(prev[robotID] || {}),
              [sensorName]: value,
            },
          }));
        }
      } catch (err) {
        console.error('Error parsing message:', err);
      }
    };

    ws.onerror = (err) => {
      console.error('WebSocket error:', err);
    };

    const interval = setInterval(() => {
      setElapsedTime(Math.floor((Date.now() - startTime) / 1000));
    }, 100);

    return () => {
      clearInterval(interval);
      ws.close();
    };
  }, []);

  const isLowBad = (sensorName) =>
    sensorName === 'battery' ||
    sensorName === 'proximity' ||
    sensorName === 'scan_accuracy';

  const checkForAlerts = (sensors) => {
    if (!sensors) return false;
    for (const [sensorName, threshold] of Object.entries(CRITICAL_THRESHOLDS)) {
      const value = sensors[sensorName];
      if (value !== undefined) {
        if (isLowBad(sensorName)) {
          if (value <= threshold.critical) return true;
        } else {
          if (value >= threshold.critical) return true;
        }
      }
    }
    return false;
  };

  const computeStatus = (sensors) => {
    if (!sensors) return 'ONLINE';
    // OFFLINE if all sensors zero (indicates crash)
    const allZero = Object.values(sensors).every((v) => v === 0);
    if (allZero) return 'OFFLINE';
    return checkForAlerts(sensors) ? 'ALERT' : 'ONLINE';
  };

  return (
    <div className="app">
      <div className="header">
        <h1>⚙️ BLACKBOX FLEET MONITOR</h1>
        <div className="timer">Elapsed: {formatTime(elapsedTime)}</div>
      </div>

      <div className="fleet-grid">
        {Object.entries(ROBOT_DATA).map(([robotID, robotInfo]) => (
          <RobotCard
            key={robotID}
            robotID={robotID}
            robotInfo={robotInfo}
            sensors={robotSensors[robotID] || {}}
            status={computeStatus(robotSensors[robotID])}
          />
        ))}
      </div>
    </div>
  );
}

function RobotCard({ robotID, robotInfo, sensors, status }) {
  const isLowBad = (name) =>
    name === 'battery' ||
    name === 'proximity' ||
    name === 'scan_accuracy';

  const getSensorClass = (sensorName, value) => {
    if (value === 0 && (sensorName === 'battery' || sensorName === 'motor_temp')) {
      return 'critical';
    }
    const threshold = CRITICAL_THRESHOLDS[sensorName];
    if (threshold) {
      if (isLowBad(sensorName)) {
        if (value <= threshold.critical) return 'critical';
        if (value <= threshold.warning) return 'warning';
      } else {
        if (value >= threshold.critical) return 'critical';
        if (value >= threshold.warning) return 'warning';
      }
    }
    return 'normal';
  };

  const getMainMetrics = () => {
    const role = robotInfo.role;
    switch (role) {
      case 'Navigator':
        return [
          { label: 'Battery', value: sensors.battery, unit: '%' },
          { label: 'Proximity', value: sensors.proximity, unit: 'cm' },
          { label: 'Motor Temp', value: sensors.motor_temp, unit: '°C' },
        ];
      case 'Carrier':
        return [
          { label: 'Battery', value: sensors.battery, unit: '%' },
          { label: 'Load', value: sensors.load_weight, unit: 'kg' },
          { label: 'Motor Temp', value: sensors.motor_temp, unit: '°C' },
        ];
      case 'Scanner':
        return [
          { label: 'Battery', value: sensors.battery, unit: '%' },
          { label: 'Accuracy', value: sensors.scan_accuracy, unit: '%' },
          { label: 'Motor Temp', value: sensors.motor_temp, unit: '°C' },
        ];
      case 'ChargeRunner':
        return [
          { label: 'Battery', value: sensors.battery, unit: '%' },
          { label: 'ETA to Dock', value: sensors.eta_to_dock, unit: 's' },
          { label: 'Motor Temp', value: sensors.motor_temp, unit: '°C' },
        ];
      default:
        return [];
    }
  };

  const statusColor = {
    ONLINE: '#4ade80',
    ALERT: '#facc15',
    OFFLINE: '#ef4444',
  }[status];

  return (
    <div className="robot-card" style={{ borderColor: statusColor }}>
      <div className="card-header" style={{ backgroundColor: statusColor }}>
        <span className="robot-icon">{robotInfo.icon}</span>
        <div>
          <div className="robot-id">{robotID}</div>
          <div className="robot-name">{robotInfo.name}</div>
        </div>
        <div className="status-badge" style={{ backgroundColor: statusColor }}>
          {status}
        </div>
      </div>

      <div className="card-body">
        {getMainMetrics().map((metric, idx) => (
          <div key={idx} className="metric">
            <div className="metric-label">{metric.label}</div>
            <div className={`metric-value ${getSensorClass(metric.label.toLowerCase().replace(' ', '_'), metric.value)}`}>
              {metric.value !== undefined ? metric.value.toFixed(1) : '—'} {metric.unit}
            </div>
          </div>
        ))}
      </div>

      <div className="card-footer">
        <div className="role-badge">{robotInfo.role}</div>
      </div>
    </div>
  );
}

function formatTime(seconds) {
  const mins = Math.floor(seconds / 60);
  const secs = seconds % 60;
  return `${mins}:${secs.toString().padStart(2, '0')}`;
}

export default App;
