async function fetchJSONFile(filePath) {
    try {
        const response = await fetch(filePath);
        const data = await response.json();
        return data;
    } catch (error) {
        console.error('Error fetching JSON file:', error);
    }
}

function processJSONData(data) {
    const revisions = [];
    const mseValues = [];
    const mapeValues = [];
    const labels = [];

    data.build_info.forEach(buildInfo => {
        const buildInfoObj = JSON.parse(buildInfo);
        const revision = buildInfoObj.revision;
        if (!revisions.includes(revision)) {
            revisions.push(revision);
        }
    });

    data.result.forEach(result => {
        labels.push(result['metric-name']);
        mseValues.push(parseFloat(result.value.mse));
        mapeValues.push(parseFloat(result.value.mape));
    });

    return { revisions, mseValues, mapeValues, labels };
}

function plotChart(revisions, mseValues, mapeValues, labels) {
    const ctx = document.getElementById('myChart').getContext('2d');
    new Chart(ctx, {
        type: 'line',
        data: {
            labels: labels,
            datasets: [{
                label: `MSE - ${revisions.join(', ')}`,
                data: mseValues,
                borderColor: 'rgba(75, 192, 192, 1)',
                backgroundColor: 'rgba(75, 192, 192, 0.2)',
                fill: false
            }, {
                label: `MAPE - ${revisions.join(', ')}`,
                data: mapeValues,
                borderColor: 'rgba(255, 99, 132, 1)',
                backgroundColor: 'rgba(255, 99, 132, 0.2)',
                fill: false
            }]
        },
        options: {
            responsive: true,
            scales: {
                x: {
                    beginAtZero: true
                },
                y: {
                    beginAtZero: true,
                    title: {
                        display: true,
                        text: 'Values'
                    }
                }
            },
            plugins: {
                legend: {
                    display: true,
                    position: 'top'
                },
                tooltip: {
                    callbacks: {
                        label: function(context) {
                            let label = context.dataset.label || '';
                            if (label) {
                                label += ': ';
                            }
                            if (context.parsed.y !== null) {
                                label += new Intl.NumberFormat('en-US', { maximumFractionDigits: 2 }).format(context.parsed.y);
                            }
                            return label;
                        }
                    }
                }
            }
        }
    });
}

function displayBuildInfo(data) {
    const buildInfoContainer = document.getElementById('build-info').querySelector('ul');
    data.build_info.forEach(info => {
        const buildInfoObj = JSON.parse(info);
        const listItem = document.createElement('li');
        listItem.textContent = `Revision: ${buildInfoObj.revision}, Version: ${buildInfoObj.version}`;
        buildInfoContainer.appendChild(listItem);
    });
}

function displayMachineSpec(data) {
    const machineSpecContainer = document.getElementById('machine-spec').querySelector('ul');
    data.machine_spec.forEach(spec => {
        const listItem = document.createElement('li');
        listItem.innerHTML = `
            <strong>Type:</strong> ${spec.type}<br>
            <strong>Model:</strong> ${spec.model}<br>
            <strong>Cores:</strong> ${spec.cores}<br>
            <strong>Threads:</strong> ${spec.threads}<br>
            <strong>Sockets:</strong> ${spec.sockets}<br>
            <strong>DRAM:</strong> ${spec.dram}
        `;
        machineSpecContainer.appendChild(listItem);
    });
}

// Path to the single JSON file
const filePath = '/tmp/v0.2-2315-g859d9bf1/v0.2-2316-gc607a6df.json';  // Change to the actual path of your JSON file

fetchJSONFile(filePath)
    .then(data => {
        const { revisions, mseValues, mapeValues, labels } = processJSONData(data);
        plotChart(revisions, mseValues, mapeValues, labels);
        displayBuildInfo(data);
        displayMachineSpec(data);
    })
    .catch(error => console.error('Error processing data:', error));
